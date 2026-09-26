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

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/result"
	"github.com/gabrielima7/GopherCore/retry"
	"go.uber.org/goleak"
)

func TestArchitecturalChaos(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	)

	var requestCount int64

	router := httpkit.NewRouter(
		httpkit.WithRateLimit(50000, 100000), // very high limits
	)

	router.Get("/sim", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		id := r.URL.Query().Get("id")
		if id == "fail" {
			httpkit.Error(w, http.StatusInternalServerError, "chaos failure")
			return
		}

		// Simulate network latency occasionally
		if atomic.LoadInt64(&requestCount)%50 == 0 {
			time.Sleep(10 * time.Millisecond)
		}

		httpkit.JSON(w, http.StatusOK, map[string]string{"msg": "success", "id": id})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()
	defer srv.Client().CloseIdleConnections()

	cache := cachekit.NewInMemoryCache(1 * time.Second)
	defer func() { _ = cache.Close() }()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())
	client := srv.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	const numItems = 25000
	const concurrencyLimit = 1000

	items := make([]int, numItems)
	for i := 0; i < numItems; i++ {
		items[i] = i
	}

	results, err := async.Map(ctx, items, concurrencyLimit, func(ctx context.Context, item int) (string, error) {
		cacheKey := "item_sim"

		if item%100 == 0 {
			var cancelFn context.CancelFunc
			ctx, cancelFn = context.WithCancel(ctx)
			cancelFn()
		}

		if _, err := cache.Get(ctx, cacheKey); err == nil {
			return "hit", nil
		}

		res := result.Of(retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
			var val string
			err := cb.ExecuteContext(ctx, func() error {
				endpoint := srv.URL + "/sim?id=ok"
				if item%15 == 0 {
					endpoint = srv.URL + "/sim?id=fail"
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
					return errors.New("simulated bad status")
				}

				val = "fetched"
				return nil
			})
			return val, err
		}, retry.WithMaxAttempts(3), retry.WithInitialDelay(2*time.Millisecond)))

		if res.IsOk() {
			val, _ := res.Unwrap()
			_ = cache.Set(ctx, cacheKey, []byte(val), 5*time.Millisecond)
			return val, nil
		}

		return "", res.Error()
	})

	if len(results) != numItems {
		t.Fatalf("expected %d results, got %d", numItems, len(results))
	}

	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "simulated bad status") &&
			!strings.Contains(errMsg, "circuit is open") &&
			!strings.Contains(errMsg, "context canceled") &&
			!errors.Is(err, context.DeadlineExceeded) &&
			!errors.Is(err, context.Canceled) {
			t.Errorf("unexpected catastrophic error: %v", err)
		}
	}
}
