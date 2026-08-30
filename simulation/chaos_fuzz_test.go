package simulation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/retry"
)

func FuzzChaos(f *testing.F) {
	defer goleak.VerifyNone(f, goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"), goleak.IgnoreTopFunction("os/signal.NotifyContext.func1"))

	router := httpkit.NewRouter(
		httpkit.WithRateLimit(50000, 100000),
		httpkit.WithCORS("*"),
	)
	router.Get("/data", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "fail" {
			httpkit.Error(w, http.StatusInternalServerError, "simulated failure")
			return
		}
		httpkit.JSON(w, http.StatusOK, map[string]string{"status": "ok", "id": id})
	})
	srv := httptest.NewServer(router)
	defer srv.Close()

	cache := cachekit.NewInMemoryCache(1 * time.Second)
	defer func() { _ = cache.Close() }()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())

	f.Add(100)
	f.Fuzz(func(t *testing.T, numRequests int) {
		if numRequests < 0 {
			numRequests = ^numRequests
		}
		numRequests = (numRequests % 2000) + 1

		startCh := make(chan struct{})
		var wg sync.WaitGroup

		errs := make([]error, numRequests)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-startCh

				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				if idx%10 == 0 {
					cancel()
				} else {
					defer cancel()
				}

				key := "req_data"
				if _, err := cache.Get(ctx, key); err == nil {
					return
				}

				res, err := retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
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
							return errors.New("bad status")
						}
						finalVal = "success"
						return nil
					})
					return finalVal, err
				}, retry.WithMaxAttempts(3), retry.WithInitialDelay(10*time.Millisecond))

				if err == nil {
					_ = cache.Set(ctx, key, []byte(res), 5*time.Second)
				} else {
					errs[idx] = err
				}
			}(i)
		}

		g := async.NewGroup()
		for i := 0; i < 50; i++ {
			g.Go(func() error {
				<-startCh
				time.Sleep(10 * time.Millisecond)
				return nil
			})
		}

		close(startCh)
		wg.Wait()
		_ = g.Wait()

		for _, err := range errs {
			if err != nil && !strings.Contains(err.Error(), "bad status") && !strings.Contains(err.Error(), "circuit is open") && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
				t.Errorf("unexpected error in chaos fuzz test: %v", err)
			}
		}
	})
}
