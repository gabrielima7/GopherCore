package simulation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/retry"
)

func TestUltimateChaosSimulation(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"))

	// 1. HTTP Server with error injections and delays
	router := httpkit.NewRouter()
	var requestCount int64
	router.Get("/chaos", func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt64(&requestCount, 1)

		// 30% chance to fail
		if count%3 == 0 {
			httpkit.Error(w, http.StatusInternalServerError, "chaos failure")
			return
		}

		// 10% chance to delay
		if count%10 == 0 {
			time.Sleep(10 * time.Millisecond)
		}

		httpkit.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	// 2. Cache
	cache := cachekit.NewInMemoryCache(1 * time.Second)
	defer func() { _ = cache.Close() }()

	// 3. Circuit Breaker
	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())

	// 4. Concurrency via Async
	const numGoroutines = 10000
	group := async.NewGroup()
	startSignal := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		idx := i
		group.Go(func() error {
			<-startSignal

			// Randomly cancel context
			var ctx context.Context
			var cancel context.CancelFunc

			if idx%5 == 0 {
				ctx, cancel = context.WithTimeout(context.Background(), 2*time.Millisecond) // highly likely to timeout
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			}
			defer cancel()

			key := "req_key"

			// Try getting from cache
			if _, err := cache.Get(ctx, key); err == nil {
				return nil
			}

			// Retry with circuit breaker
			_, err := retry.DoWithValue(ctx, func(c context.Context) (string, error) {
				var result string

				execErr := cb.ExecuteContext(c, func() error {
					req, reqErr := http.NewRequestWithContext(c, "GET", srv.URL+"/chaos", nil)
					if reqErr != nil {
						return reqErr
					}

					resp, doErr := http.DefaultClient.Do(req)
					if doErr != nil {
						return doErr
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						return errors.New("bad status")
					}
					result = "success"
					return nil
				})
				return result, execErr
			}, retry.WithMaxAttempts(2), retry.WithInitialDelay(1*time.Millisecond))

			// Simulate setting cache on success
			if err == nil {
				_ = cache.Set(ctx, key, []byte("success"), 5*time.Second)
			}

			return err
		})
	}

	close(startSignal) // Unleash the chaos

	errs := group.Wait()

	// 5. Validation
	for _, err := range errs {
		if err != nil {
			msg := err.Error()
			if !strings.Contains(msg, "circuit is open") &&
				!strings.Contains(msg, "bad status") &&
				!strings.Contains(msg, "too many requests") &&
				!errors.Is(err, context.DeadlineExceeded) &&
				!errors.Is(err, context.Canceled) &&
				!strings.Contains(msg, "context deadline exceeded") &&
				!strings.Contains(msg, "context canceled") &&
				!strings.Contains(msg, "connection refused") &&
				!strings.Contains(msg, "EOF") &&
				!strings.Contains(msg, "connection reset by peer") &&
				!strings.Contains(msg, "Client.Timeout exceeded while awaiting headers") {
				t.Errorf("Unexpected error during massive chaos load: %v", err)
			}
		}
	}
}
