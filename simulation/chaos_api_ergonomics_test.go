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

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/dbkit"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/logkit"
	"github.com/gabrielima7/GopherCore/result"
	"github.com/gabrielima7/GopherCore/retry"
	"go.uber.org/goleak"
)

func TestAPI_ErgonomicsAndChaos(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
		goleak.IgnoreTopFunction("os/signal.NotifyContext.func1"),
	)

	logkit.Initialize()

	// 1. Setup Database
	dbPath := filepath.Join(t.TempDir(), "api_ergo_chaos.db")
	db, err := dbkit.Connect(context.Background(), "sqlite3", dbPath)
	if err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skip("skipping test: sqlite3 requires cgo")
		}
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS ergo_metrics (id INTEGER PRIMARY KEY, hits INTEGER)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// 2. Setup Cache and Circuit Breaker
	cache := cachekit.NewInMemoryCache(2 * time.Second)
	defer cache.Close()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())

	// 3. Setup HTTP Server
	router := httpkit.NewRouter(httpkit.WithRateLimit(1000000, 2000000))
	var httpHits atomic.Int64
	var dbErrorCount atomic.Int64
	var successCount atomic.Int64

	router.Get("/ergo", func(w http.ResponseWriter, r *http.Request) {
		httpHits.Add(1)

		ctx := r.Context()
		if r.URL.Query().Get("fail") == "true" {
			httpkit.Error(w, http.StatusInternalServerError, "http simulated failure")
			return
		}
		if r.URL.Query().Get("panic") == "true" {
			panic("http simulated panic")
		}

		// Read/Write DB
		_, dbErr := db.ExecContext(ctx, "INSERT INTO ergo_metrics (hits) VALUES (1)")
		if dbErr != nil {
			if strings.Contains(dbErr.Error(), "database is locked") || strings.Contains(dbErr.Error(), "SQLITE_BUSY") || strings.Contains(dbErr.Error(), "busy") {
				dbErrorCount.Add(1)
				httpkit.Error(w, http.StatusTooManyRequests, "db busy")
				return
			}
			dbErrorCount.Add(1)
			httpkit.Error(w, http.StatusInternalServerError, "db error")
			return
		}

		successCount.Add(1)
		httpkit.Ok(w, map[string]string{"msg": "http_success"})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()
	defer srv.Client().CloseIdleConnections()

	httpClient := srv.Client()

	// 4. Unleash Chaos with 5000 concurrent goroutines
	const numGoroutines = 5000
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var reqs []int
	for i := 0; i < numGoroutines; i++ {
		reqs = append(reqs, i)
	}

	var clientSuccessCount atomic.Int64
	var genericErrors atomic.Int64
	var cbErrors atomic.Int64

	_, _ = async.Map(ctx, reqs, 1000, func(c context.Context, id int) (string, error) {
		key := "ergo_key"
		// 30% chance to check cache and hit directly
		if id%3 == 0 {
			if _, cErr := cache.Get(c, key); cErr == nil {
				clientSuccessCount.Add(1)
				return "cached", nil
			}
		}

		// Ergonomically wrap the network interaction in result/retry/circuitbreaker
		res := result.Of(retry.DoWithValue(c, func(rc context.Context) (string, error) {
			var finalMsg string
			cbErr := cb.ExecuteContext(rc, func() error {
				endpoint := srv.URL + "/ergo"
				if id%50 == 0 {
					endpoint += "?panic=true"
				} else if id%30 == 0 {
					endpoint += "?fail=true"
				}

				req, reqErr := http.NewRequestWithContext(rc, http.MethodGet, endpoint, nil)
				if reqErr != nil {
					return reqErr
				}
				resp, doErr := httpClient.Do(req)
				if doErr != nil {
					return doErr
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					if resp.StatusCode == http.StatusTooManyRequests {
						return errors.New("http too many requests")
					}
					return errors.New("http bad status")
				}
				finalMsg = "full_success"
				return nil
			})
			if cbErr != nil {
				return "", cbErr
			}
			return finalMsg, nil
		}, retry.WithMaxAttempts(3)))

		if res.IsErr() {
			err := res.Error()
			if strings.Contains(err.Error(), "circuit is open") || strings.Contains(err.Error(), "too many requests") || strings.Contains(err.Error(), "http too many requests") {
				cbErrors.Add(1)
			} else {
				genericErrors.Add(1)
			}
			return "", err
		}

		val, _ := res.Unwrap()
		_ = cache.Set(c, key, []byte(val), 100*time.Millisecond)
		clientSuccessCount.Add(1)
		return val, nil
	})

	// 5. Verification Constraints
	if clientSuccessCount.Load() == 0 && genericErrors.Load() > 0 {
		t.Fatalf("Chaos test failed completely: 0 successes, %d errors", genericErrors.Load())
	}
	if httpHits.Load() == 0 {
		t.Fatalf("HTTP server received 0 hits, test is invalid")
	}

	// Validate DB state to ensure concurrency worked
	var dbCount int
	if err := db.Get(&dbCount, "SELECT COUNT(*) FROM ergo_metrics"); err != nil {
		t.Fatalf("failed to query database at end of test: %v", err)
	}

	if int64(dbCount) != successCount.Load() {
		t.Errorf("Database records (%d) do not match server successful inserts (%d)", dbCount, successCount.Load())
	}
}
