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

func TestFullSystemChaos(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"), goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"))

	// 1. Setup SQLite Database
	dbPath := filepath.Join(t.TempDir(), "full_system_chaos.db")
	db, err := dbkit.Connect(context.Background(), "sqlite3", dbPath)
	if err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skip("skipping test: sqlite3 requires cgo, but CGO_ENABLED=0")
		}
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS sys_data (id INTEGER PRIMARY KEY, value TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// 2. Setup HTTP Server
	router := httpkit.NewRouter(
		httpkit.WithRateLimit(100000, 200000),
		httpkit.WithCORS("*"),
	)

	var requestCount int64
	router.Get("/system", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		id := r.URL.Query().Get("id")
		if id == "crash" {
			httpkit.Error(w, http.StatusInternalServerError, "simulated backend crash")
			return
		}

		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM sys_data")
		if err != nil {
			httpkit.Error(w, http.StatusInternalServerError, "db error")
			return
		}

		httpkit.JSON(w, http.StatusOK, map[string]string{"msg": "success", "id": id})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()
	defer srv.Client().CloseIdleConnections()

	// 3. Cache & Circuit Breaker
	cache := cachekit.NewInMemoryCache(1 * time.Second)
	defer func() { _ = cache.Close() }()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())
	client := srv.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const numItems = 5000
	const concurrencyLimit = 500

	items := make([]int, numItems)
	for i := 0; i < numItems; i++ {
		items[i] = i
	}

	// 4. Execution
	resList, err := async.Map(ctx, items, concurrencyLimit, func(ctx context.Context, item int) (string, error) {
		key := "sys_item_data"

		// Simulate random context cancellations
		if item%500 == 0 {
			var cancelFn context.CancelFunc
			ctx, cancelFn = context.WithCancel(ctx)
			cancelFn()
		} else if item%700 == 0 {
			var cancelFn context.CancelFunc
			ctx, cancelFn = context.WithTimeout(ctx, 1*time.Millisecond)
			defer cancelFn()
		}

		// Try Cache First
		if _, err := cache.Get(ctx, key); err == nil {
			return "hit", nil
		}

		res := result.Of(retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
			var val string
			err := cb.ExecuteContext(ctx, func() error {
				endpoint := srv.URL + "/system?id=ok"
				if item%10 == 0 {
					endpoint = srv.URL + "/system?id=crash"
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
		}, retry.WithMaxAttempts(3), retry.WithInitialDelay(5*time.Millisecond)))

		if res.IsOk() {
			val, _ := res.Unwrap()
			_ = cache.Set(ctx, key, []byte(val), 2*time.Second)
			return val, nil
		}

		return "", res.Error()
	})

	// 5. Validation
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
			t.Errorf("unexpected error during full system chaos load: %v", err)
		}
	}
}
