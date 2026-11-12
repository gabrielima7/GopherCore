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

func TestApiErgonomicsSimulation(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"), goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"))

	router := httpkit.NewRouter(httpkit.WithRateLimit(1000000, 2000000))
	var ops atomic.Int64
	router.Get("/ergonomics", func(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const numItems = 20000
	const concurrencyLimit = 1500

	items := make([]int, numItems)
	for i := 0; i < numItems; i++ {
		items[i] = i
	}

	resList, err := async.Map(ctx, items, concurrencyLimit, func(ctx context.Context, item int) (string, error) {
		key := "ergo_data"

		if item%500 == 0 {
			var cancelFn context.CancelFunc
			ctx, cancelFn = context.WithCancel(ctx)
			cancelFn()
		} else if item%700 == 0 {
			var cancelFn context.CancelFunc
			ctx, cancelFn = context.WithTimeout(ctx, 1*time.Millisecond)
			defer cancelFn()
		}

		if _, err := cache.Get(ctx, key); err == nil {
			return "hit", nil
		}

		res := result.Of(retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
			var val string
			err := cb.ExecuteContext(ctx, func() error {
				endpoint := srv.URL + "/ergonomics"
				if item%10 == 0 {
					endpoint = srv.URL + "/ergonomics?boom=true"
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
		}, retry.WithMaxAttempts(3), retry.WithInitialDelay(1*time.Millisecond)))

		if res.IsOk() {
			val, _ := res.Unwrap()
			_ = cache.Set(ctx, key, []byte(val), 2*time.Second)
			return val, nil
		}

		return "", res.Error()
	})

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
