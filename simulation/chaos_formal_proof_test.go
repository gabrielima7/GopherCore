package simulation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/result"
	"github.com/gabrielima7/GopherCore/retry"
)

func TestFormalProofSimulation(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"), goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"))

	// 1. Setup HTTP Server with middlewares
	router := httpkit.NewRouter(
		httpkit.WithRateLimit(100000, 200000), // High limit to bypass basic rejection
	)

	var requestCount int64
	router.Get("/proof", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		if r.URL.Query().Get("fail") == "true" {
			httpkit.Error(w, http.StatusInternalServerError, "simulated 500 error")
			return
		}
		httpkit.JSON(w, http.StatusOK, map[string]interface{}{"msg": "formal_proof", "status": "ok"})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()
	defer srv.Client().CloseIdleConnections()

	// 2. Cache & Circuit Breaker Setup
	cache := cachekit.NewInMemoryCache(1 * time.Second)
	defer func() { _ = cache.Close() }()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())
	client := srv.Client()

	const numGoroutines = 10000
	group := async.NewGroup()
	startSignal := make(chan struct{})

	// 3. Launch massive concurrent attack
	for i := 0; i < numGoroutines; i++ {
		idx := i
		group.Go(func() error {
			<-startSignal

			// 3a. Context Cancellation
			ctx := context.Background()
			var cancel context.CancelFunc
			if idx%20 == 0 {
				ctx, cancel = context.WithTimeout(ctx, 1*time.Microsecond)
			} else {
				ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
			}
			defer cancel()

			key := "proof_key"

			// Evaluate API Ergonomics: Is the use of the `result` pattern idiomatic in Go?
			// Wrapping cache fetch inside the Result pattern for safe functional pipelining
			cachedRes := result.Of(cache.Get(ctx, key))
			if cachedRes.IsOk() {
				return nil // Cache hit, exit early
			}

			// 3b. Integration: Circuitbreaker wrapped in Retry wrapped in Result
			rawRes, retryErr := retry.DoWithValue(ctx, func(c context.Context) (string, error) {
				var finalVal string
				err := cb.ExecuteContext(c, func() error {
					endpoint := srv.URL + "/proof"
					if idx%5 == 0 {
						endpoint += "?fail=true" // Induce chaos/failures
					}

					req, reqErr := http.NewRequestWithContext(c, "GET", endpoint, nil)
					if reqErr != nil {
						return reqErr
					}

					resp, doErr := client.Do(req)
					if doErr != nil {
						return doErr
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						return errors.New("bad status")
					}
					finalVal = "success"
					return nil
				})
				return finalVal, err
			}, retry.WithMaxAttempts(3), retry.WithInitialDelay(5*time.Millisecond))

			// Convert classic (value, err) to idiomatic Monadic Result container for safety
			res := result.Of(rawRes, retryErr)

			if res.IsOk() {
				val, _ := res.Unwrap() // Safe because IsOk was checked
				_ = cache.Set(ctx, key, []byte(val), 2*time.Second)
				return nil
			}

			return res.Error()
		})
	}

	// 4. Release all goroutines simultaneously to enforce extreme CPU/Memory contention
	close(startSignal)
	errs := group.Wait()

	// 5. Verification - Assert no uncaught panics, data races, or unexpected errors
	for _, err := range errs {
		if err != nil {
			msg := err.Error()
			// We expect these errors under chaos simulation. Any other error means a system bug.
			if !errors.Is(err, circuitbreaker.ErrCircuitOpen) &&
				msg != "bad status" &&
				!errors.Is(err, circuitbreaker.ErrTooManyRequests) &&
				msg != "circuit is open" &&
				!errors.Is(err, context.DeadlineExceeded) &&
				!errors.Is(err, context.Canceled) &&
				!errors.Is(err, retry.ErrMaxAttemptsReached) {
				t.Errorf("Unexpected error during formal proof massive load: %v", err)
			}
		}
	}
}
