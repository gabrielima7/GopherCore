package simulation

import (
	"go.uber.org/goleak"

	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/retry"
)

func TestExtremeConcurrencyLoad(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"))

	router := httpkit.NewRouter()
	var requestCount int64
	router.Get("/process", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		if r.URL.Query().Get("fail") == "true" {
			httpkit.Error(w, http.StatusInternalServerError, "simulated chaos failure")
			return
		}
		httpkit.JSON(w, http.StatusOK, map[string]string{"status": "success"})
	})
	srv := httptest.NewServer(router)
	defer srv.Close()

	cache := cachekit.NewInMemoryCache(1 * time.Second)
	defer func() { _ = cache.Close() }()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())

	const numGoroutines = 5000
	group := async.NewGroup()
	startSignal := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		idx := i
		group.Go(func() error {
			<-startSignal

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			key := "req_key"
			if _, err := cache.Get(ctx, key); err == nil {
				return nil
			}

			_, err := retry.DoWithValue(ctx, func(c context.Context) (string, error) {
				var result string
				execErr := cb.Execute(func() error {
					endpoint := srv.URL + "/process"
					if idx%3 == 0 {
						endpoint += "?fail=true"
					}

					req, reqErr := http.NewRequestWithContext(c, "GET", endpoint, nil)
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
			}, retry.WithMaxAttempts(3), retry.WithInitialDelay(5*time.Millisecond))

			if err == nil {
				_ = cache.Set(ctx, key, []byte("success"), 5*time.Second)
			}
			return err
		})
	}

	close(startSignal) // Unleash all goroutines at once

	errs := group.Wait()

	for _, err := range errs {
		if err != nil {
			msg := err.Error()
			if !strings.Contains(msg, "circuit is open") &&
				!strings.Contains(msg, "bad status") &&
				!strings.Contains(msg, "too many requests") &&
				!errors.Is(err, context.DeadlineExceeded) &&
				!errors.Is(err, context.Canceled) {
				t.Errorf("Unexpected error during massive concurrency load: %v", err)
			}
		}
	}
}
