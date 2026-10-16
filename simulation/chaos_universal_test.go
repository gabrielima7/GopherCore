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
	"github.com/gabrielima7/GopherCore/result"
	"github.com/gabrielima7/GopherCore/retry"
)

func TestUniversalChaosSimulation(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"), goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"))

	router := httpkit.NewRouter(httpkit.WithRateLimit(1000000, 2000000))
	var ops atomic.Int64
	router.Get("/chaos", func(w http.ResponseWriter, r *http.Request) {
		ops.Add(1)
		if r.URL.Query().Get("boom") == "true" {
			httpkit.Error(w, http.StatusInternalServerError, "simulated chaos")
			return
		}
		httpkit.JSON(w, http.StatusOK, map[string]string{"state": "stable"})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	cache := cachekit.NewInMemoryCache(10 * time.Millisecond)
	defer func() { _ = cache.Close() }()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())
	client := srv.Client()
	if tr, ok := client.Transport.(*http.Transport); ok {
		tr.MaxIdleConns = 20000
		tr.MaxIdleConnsPerHost = 20000
	}
	defer client.CloseIdleConnections()

	numRoutines := 20000
	if testing.Short() {
		numRoutines = 2000
	}
	startSignal := make(chan struct{})
	group := async.NewGroup()

	for i := 0; i < numRoutines; i++ {
		id := i
		group.Go(func() error {
			ctx := context.Background()
			<-startSignal

			key := "key_universal"

			if _, err := cache.Get(ctx, key); err == nil {
				return nil
			}

			res := result.Of(retry.DoWithValue(ctx, func(c context.Context) (string, error) {
				var final string
				cbErr := cb.ExecuteContext(c, func() error {
					endpoint := srv.URL + "/chaos"
					if id%10 == 0 {
						endpoint += "?boom=true"
					}
					req, reqErr := http.NewRequestWithContext(c, http.MethodGet, endpoint, nil)
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
					final = "success"
					return nil
				})
				if cbErr != nil {
					return "", cbErr
				}
				return final, nil
			}, retry.WithMaxAttempts(3), retry.WithInitialDelay(1*time.Millisecond)))

			if res.Error() == nil {
				_ = cache.Set(ctx, key, []byte(res.UnwrapOr("")), 5*time.Second)
			}
			return res.Error()
		})
	}

	close(startSignal)
	errs := group.Wait()

	for _, err := range errs {
		if err != nil {
			msg := err.Error()
			if !strings.Contains(msg, "circuit is open") &&
				!strings.Contains(msg, "bad status") &&
				!strings.Contains(msg, "too many requests") &&
				!strings.Contains(msg, "can't assign requested address") &&
				!strings.Contains(msg, "dial tcp") &&
				!strings.Contains(msg, "max attempts reached") &&
				!strings.Contains(msg, "connection refused") &&
				!strings.Contains(msg, "connection reset by peer") &&
				!strings.Contains(msg, "EOF") &&
				!errors.Is(err, context.DeadlineExceeded) &&
				!errors.Is(err, context.Canceled) {
				t.Errorf("Unexpected error in Universal Chaos: %v", err)
			}
		}
	}
}
