package simulation

import (
	"context"
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

// FuzzLeakCheck ensures that dynamic load injections do not trigger goroutine leaks, context timeouts,
// or memory escapes across the core architecture.
func FuzzLeakCheck(f *testing.F) {
	f.Add(100)
	f.Add(500)
	f.Add(1000)
	f.Add(5000)

	f.Fuzz(func(t *testing.T, concurrencyLimit int) {
		if concurrencyLimit < 10 || concurrencyLimit > 5000 {
			t.Skip("concurrency limit out of bounds for fast fuzzer")
		}

		defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"), goleak.IgnoreTopFunction("os/signal.NotifyContext.func1"), goleak.IgnoreTopFunction("testing.(*F).Fuzz.func1"))

		router := httpkit.NewRouter(httpkit.WithRateLimit(10000, 20000))
		var requestCount int64
		router.Get("/fuzz", func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt64(&requestCount, 1)
			httpkit.JSON(w, http.StatusOK, map[string]string{"msg": "fuzzed"})
		})

		srv := httptest.NewServer(router)
		defer srv.Close()
		client := srv.Client()
		defer client.CloseIdleConnections()

		cb := circuitbreaker.New(circuitbreaker.DefaultConfig())
		cache := cachekit.NewInMemoryCache(10 * time.Millisecond)
		defer func() { _ = cache.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		items := make([]int, concurrencyLimit)
		for i := 0; i < concurrencyLimit; i++ {
			items[i] = i
		}

		_, _ = async.Map(ctx, items, concurrencyLimit/2+1, func(ctx context.Context, item int) (string, error) {
			key := "fuzz_key"
			if item%10 == 0 {
				var cancelFn context.CancelFunc
				ctx, cancelFn = context.WithCancel(ctx)
				cancelFn()
			}
			if _, err := cache.Get(ctx, key); err == nil {
				return "hit", nil
			}

			res := result.Of(retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
				var val string
				err := cb.ExecuteContext(ctx, func() error {
					req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/fuzz", nil)
					if err != nil {
						return err
					}
					resp, err := client.Do(req)
					if err != nil {
						return err
					}
					defer resp.Body.Close()
					val = "ok"
					return nil
				})
				return val, err
			}, retry.WithMaxAttempts(2)))

			if res.IsOk() {
				val, _ := res.Unwrap()
				_ = cache.Set(ctx, key, []byte(val), 2*time.Second)
				return val, nil
			}
			return "", res.Error()
		})
	})
}
