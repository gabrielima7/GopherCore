package simulation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/retry"
)

var errSimulated = errors.New("simulated error")

func TestArchitectureSimulation_HighDemand(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"), goleak.IgnoreTopFunction("os/signal.NotifyContext.func1"))

	router := httpkit.NewRouter(
		httpkit.WithRateLimit(500000, 1000000), // very high limits
	)
	router.Get("/api/v1/resource", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "crash" {
			httpkit.Error(w, http.StatusInternalServerError, "simulated crash")
			return
		}
		httpkit.JSON(w, http.StatusOK, map[string]string{"status": "ok", "id": id})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()
	defer srv.Client().CloseIdleConnections()

	cache := cachekit.NewInMemoryCache(10 * time.Millisecond)
	defer func() { _ = cache.Close() }()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())
	client := srv.Client()

	numGoroutines := 2000
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	errs := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startCh

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if idx%20 == 0 {
				cancel() // immediately cancel to test context cancellation
			} else {
				defer cancel()
			}

			key := "key_shared"

			// Simulate read from cache
			if val, err := cache.Get(ctx, key); err == nil && len(val) > 0 {
				return
			}

			// Simulated fallback using retry and circuitbreaker
			res, err := retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
				var finalVal string
				cbErr := cb.ExecuteContext(ctx, func() error {
					endpoint := srv.URL + "/api/v1/resource?id=ok"
					if idx%7 == 0 {
						endpoint = srv.URL + "/api/v1/resource?id=crash"
					}
					req, reqErr := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
					if reqErr != nil {
						return reqErr
					}
					resp, respErr := client.Do(req)
					if respErr != nil {
						return respErr
					}
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						return errSimulated
					}
					finalVal = "success_data"
					return nil
				})
				return finalVal, cbErr
			}, retry.WithMaxAttempts(2), retry.WithInitialDelay(5*time.Millisecond))

			if err == nil {
				_ = cache.Set(ctx, key, []byte(res), 50*time.Millisecond)
			} else {
				errs[idx] = err
			}
		}(i)
	}

	close(startCh)
	wg.Wait()
}
