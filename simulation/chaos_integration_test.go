package simulation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/dbkit"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/result"
	"github.com/gabrielima7/GopherCore/retry"
	_ "github.com/mattn/go-sqlite3"
)

var errBadStatus = errors.New("bad status")

func TestIntegrationChaos(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "integration_chaos.db")
	db := dbkit.MustConnect(context.Background(), "sqlite3", dbPath)
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS data (id INTEGER PRIMARY KEY, value TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	router := httpkit.NewRouter(
		httpkit.WithRateLimit(50000, 100000), // High rate limit to allow load
		httpkit.WithCORS("*"),
	)

	router.Get("/data", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "fail" {
			httpkit.Error(w, http.StatusInternalServerError, "simulated failure")
			return
		}

		var count int
		err := db.Get(&count, "SELECT COUNT(*) FROM data")
		if err != nil {
			httpkit.Error(w, http.StatusInternalServerError, "db error")
			return
		}

		httpkit.JSON(w, http.StatusOK, map[string]string{"status": "ok", "id": id})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	cache := cachekit.NewInMemoryCache(1 * time.Second)
	defer cache.Close()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())

	const numRequests = 5000

	startCh := make(chan struct{})
	var wg sync.WaitGroup

	errs := make([]error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startCh

			// Simulate context cancellation for some requests
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			if idx%10 == 0 {
				cancel()
			} else {
				defer cancel()
			}

			// 1. Try Cache First
			key := "req_data"
			if _, err := cache.Get(ctx, key); err == nil {
				// Cache hit
				return
			}

			// 2. Wrap network call in Retry & CircuitBreaker
			res := result.Of(retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
				var finalVal string
				err := cb.Execute(func() error {
					endpoint := srv.URL + "/data?id=ok"
					if idx%5 == 0 {
						endpoint = srv.URL + "/data?id=fail"
					}

					req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
					if err != nil {
						return err
					}

					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						return err
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						return errBadStatus
					}

					finalVal = "success"
					return nil
				})
				return finalVal, err
			}, retry.WithMaxAttempts(3), retry.WithInitialDelay(10*time.Millisecond)))

			// 3. Cache the result if successful
			if res.IsOk() {
				val, _ := res.Unwrap()
				_ = cache.Set(ctx, key, []byte(val), 5*time.Second)
			} else {
				errs[idx] = res.Error()
			}
		}(i)
	}

	// Use async.Group for secondary background tasks
	g := async.NewGroup()
	for i := 0; i < 50; i++ {
		g.Go(func() error {
			<-startCh
			// Perform some dummy async work
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}

	close(startCh)
	wg.Wait()
	_ = g.Wait()

	// Verify we handled errors gracefully (some are expected due to intentional cancellations and failures)
	// The main goal is no race conditions or deadlocks.
	for _, err := range errs {
		if err != nil && !isExpectedError(err) {
			t.Errorf("unexpected error in chaos integration test: %v", err)
		}
	}
}

func isExpectedError(err error) bool {
	if err == nil {
		return true
	}
	// Unwrap joined errors if any (from errors.Join)
	if uw, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range uw.Unwrap() {
			if !isExpectedError(e) {
				return false
			}
		}
		return true
	}
	// Check individual error
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, circuitbreaker.ErrCircuitOpen) ||
		errors.Is(err, circuitbreaker.ErrTooManyRequests) ||
		errors.Is(err, retry.ErrMaxAttemptsReached) ||
		errors.Is(err, errBadStatus) {
		return true
	}
	return false
}
