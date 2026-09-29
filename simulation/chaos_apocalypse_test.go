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
	"github.com/gabrielima7/GopherCore/guard"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/jsonutil"
	"github.com/gabrielima7/GopherCore/logkit"
	"github.com/gabrielima7/GopherCore/retry"
	_ "github.com/mattn/go-sqlite3"
)

type ApocalypsePayload struct {
	Data string `validate:"required,min=3"`
	Key  string `validate:"required"`
}

func TestApocalypseChaosSimulation(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"), goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"))

	// Initialize logkit explicitly to discard/buffer or configure it
	logkit.Initialize()

	// 1. Setup SQLite Database for extreme read/write contention
	dbPath := filepath.Join(t.TempDir(), "apocalypse_chaos.db")
	db, err := dbkit.Connect(context.Background(), "sqlite3", dbPath)
	if err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skip("skipping test: sqlite3 requires cgo, but CGO_ENABLED=0")
		}
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS chaos_metrics (id INTEGER PRIMARY KEY, value TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// 2. Setup HTTP Server with middlewares
	router := httpkit.NewRouter(
		httpkit.WithRateLimit(100000, 200000), // Huge rate limit to prevent standard rejections
		httpkit.WithCORS("*"),
	)

	var requestCount int64
	router.Post("/apocalypse", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		fail := r.URL.Query().Get("fail")
		switch fail {
		case "panic":
			panic("simulated fatal apocalypse panic")
		case "true":
			httpkit.Error(w, http.StatusInternalServerError, "simulated 500 error")
			return
		}

		var payload ApocalypsePayload
		if err := jsonutil.NewDecoder(r.Body).Decode(&payload); err != nil {
			httpkit.Error(w, http.StatusBadRequest, "invalid json")
			return
		}

		payload.Data = guard.SanitizeString(payload.Data)
		payload.Data = guard.StripHTML(payload.Data)

		if err := guard.Validate(payload); err != nil {
			httpkit.Error(w, http.StatusUnprocessableEntity, "validation failed")
			return
		}

		var count int
		if err := db.Get(&count, "SELECT COUNT(*) FROM chaos_metrics"); err != nil {
			httpkit.Error(w, http.StatusInternalServerError, "db read error")
			return
		}

		httpkit.JSON(w, http.StatusOK, map[string]interface{}{"msg": "survived", "count": count})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()
	defer srv.Client().CloseIdleConnections()

	// 3. Cache & Circuit Breaker Setup
	cache := cachekit.NewInMemoryCache(1 * time.Second)
	defer func() { _ = cache.Close() }()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())
	client := srv.Client()

	const numGoroutines = 10000
	group := async.NewGroup()
	startSignal := make(chan struct{})

	// 4. Fire massive concurrent requests
	for i := 0; i < numGoroutines; i++ {
		idx := i
		group.Go(func() error {
			<-startSignal

			// Randomize context cancellations and timeouts
			ctx := context.Background()
			var cancel context.CancelFunc
			if idx%100 == 0 {
				ctx, cancel = context.WithCancel(ctx)
				cancel() // Immediate cancellation
			} else if idx%200 == 0 {
				ctx, cancel = context.WithTimeout(ctx, 1*time.Millisecond) // Fast timeout
				defer cancel()
			} else {
				ctx, cancel = context.WithTimeout(ctx, 5*time.Second) // Generous timeout
				defer cancel()
			}

			key := "apoc_key"
			if idx%2 == 0 { // 50% chance to check cache first
				if _, err := cache.Get(ctx, key); err == nil {
					return nil
				}
			}

			_, err := retry.DoWithValue(ctx, func(c context.Context) (string, error) {
				var result string
				execErr := cb.ExecuteContext(ctx, func() error {
					endpoint := srv.URL + "/apocalypse"
					if idx%50 == 0 {
						endpoint += "?fail=panic"
					} else if idx%10 == 0 {
						endpoint += "?fail=true"
					}

					// Prepare payload. Inject bad JSON sometimes.
					payloadData := `{"Data": "safe_data", "Key": "val"}`
					if idx%30 == 0 {
						payloadData = `{"Data": "<script>alert(1)</script>", "Key": "val"}` // XSS
					} else if idx%40 == 0 {
						payloadData = `{"Data": "s"}` // Fails guard validation (min=3)
					} else if idx%70 == 0 {
						payloadData = `{invalid json`
					}

					req, reqErr := http.NewRequestWithContext(c, "POST", endpoint, strings.NewReader(payloadData))
					if reqErr != nil {
						return reqErr
					}

					resp, doErr := client.Do(req)
					if doErr != nil {
						return doErr
					}
					defer resp.Body.Close()

					if resp.StatusCode == http.StatusOK {
						result = "success"
						return nil
					}
					return errors.New("bad status")
				})
				return result, execErr
			}, retry.WithMaxAttempts(3), retry.WithInitialDelay(5*time.Millisecond))

			if err == nil {
				_ = cache.Set(ctx, key, []byte("success"), 5*time.Second)
			}
			return err
		})
	}

	close(startSignal) // Unleash the apocalypse
	errs := group.Wait()

	// 5. Validation - the system should survive without deadlocks, and return expected network/cb errors.
	for _, err := range errs {
		if err != nil {
			msg := err.Error()
			if !strings.Contains(msg, "circuit is open") &&
				!strings.Contains(msg, "bad status") &&
				!strings.Contains(msg, "too many requests") &&
				!strings.Contains(msg, "connection refused") &&
				!strings.Contains(msg, "EOF") && // Server closed connection early sometimes
				!errors.Is(err, context.DeadlineExceeded) &&
				!errors.Is(err, context.Canceled) {
				t.Errorf("Unexpected error during massive concurrency load: %v", err)
			}
		}
	}
}
