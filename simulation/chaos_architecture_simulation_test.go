package simulation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/dbkit"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/result"
	"github.com/gabrielima7/GopherCore/retry"
	_ "github.com/mattn/go-sqlite3"
)

// TestArchitectureSimulation simulates a real-world scenario architecture, evaluating API Ergonomics and Thread-Safety
// by forcing thousands of concurrent goroutines to access shared states across dbkit, httpkit, cachekit, circuitbreaker,
// and retry packages, testing their inter-operation and failure bounds under simulated load.
func TestArchitectureSimulation(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"), goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"))

	// Setup SQLite Database
	dbPath := filepath.Join(t.TempDir(), "architecture_sim.db")
	db, err := dbkit.Connect(context.Background(), "sqlite3", dbPath)
	if err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skip("skipping test: sqlite3 requires cgo, but CGO_ENABLED=0")
		}
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS sim_data (id INTEGER PRIMARY KEY, value TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Setup HTTP Server
	router := httpkit.NewRouter(
		httpkit.WithRateLimit(100000, 200000),
		httpkit.WithCORS("*"),
	)

	var requestCount int64
	router.Get("/sim", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		id := r.URL.Query().Get("id")
		if id == "crash" {
			httpkit.Error(w, http.StatusInternalServerError, "simulated backend crash")
			return
		}

		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM sim_data")
		if err != nil {
			if strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "SQLITE_BUSY") {
				httpkit.Error(w, http.StatusTooManyRequests, "database is locked")
				return
			}
			httpkit.Error(w, http.StatusInternalServerError, "db error")
			return
		}

		httpkit.JSON(w, http.StatusOK, map[string]string{"msg": "success", "id": id})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	// Cache & Circuit Breaker
	cache := cachekit.NewInMemoryCache(1 * time.Second)
	defer func() { _ = cache.Close() }()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())
	client := srv.Client()
	defer client.CloseIdleConnections()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const numItems = 2000
	const concurrencyLimit = 200

	items := make([]int, numItems)
	for i := 0; i < numItems; i++ {
		items[i] = i
	}

	resList, err := async.Map(ctx, items, concurrencyLimit, func(ctx context.Context, item int) (string, error) {
		key := "sim_item_data"

		if item%300 == 0 {
			var cancelFn context.CancelFunc
			ctx, cancelFn = context.WithCancel(ctx)
			cancelFn()
		}

		if _, err := cache.Get(ctx, key); err == nil {
			return "hit", nil
		}

		res := result.Of(retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
			var val string
			err := cb.ExecuteContext(ctx, func() error {
				endpoint := srv.URL + "/sim?id=ok"
				if item%10 == 0 {
					endpoint = srv.URL + "/sim?id=crash"
				}

				req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
				if err != nil {
					return err
				}

				resp, err := client.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					return errors.New("bad status")
				}
				val = "fetched"
				return nil
			})
			return val, err
		}, retry.WithMaxAttempts(3), retry.WithInitialDelay(1*time.Millisecond)))

		if res.IsOk() {
			val, _ := res.Unwrap()
			_ = cache.Set(ctx, key, []byte(val), 2*time.Second)
			return val, nil
		}

		return "", res.Error()
	})

	if len(resList) != numItems {
		t.Fatalf("expected %d results, got %d", numItems, len(resList))
	}

	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "bad status") &&
			!strings.Contains(errMsg, "circuit is open") &&
			!strings.Contains(errMsg, "too many requests") &&
			!strings.Contains(errMsg, "connection refused") &&
			!strings.Contains(errMsg, "connectex") &&
			!strings.Contains(errMsg, "dial tcp") &&
			!strings.Contains(errMsg, "max attempts reached") &&
			!errors.Is(err, context.DeadlineExceeded) &&
			!errors.Is(err, context.Canceled) {
			t.Errorf("unexpected error during simulation: %v", err)
		}
	}
}
