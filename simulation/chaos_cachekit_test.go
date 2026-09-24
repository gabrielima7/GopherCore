package simulation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/retry"
	"go.uber.org/goleak"
)

func TestChaosCachekitIntegration(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"))

	// 1. Setup backend
	router := httpkit.NewRouter(
		httpkit.WithRateLimit(100000, 200000),
	)
	router.Get("/process", func(w http.ResponseWriter, r *http.Request) {
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

	// 2. Setup Cache and Breaker
	cache := cachekit.NewInMemoryCache(1 * time.Second)
	defer func() { _ = cache.Close() }()
	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const numItems = 2000
	const concurrencyLimit = 200

	items := make([]int, numItems)
	for i := 0; i < numItems; i++ {
		items[i] = i
	}

	// 3. Actively execute bounded concurrency mapping
	resList, err := async.Map(ctx, items, concurrencyLimit, func(ctx context.Context, item int) (string, error) {
		// Use a shared key for some items to test cache concurrency heavily
		key := "item_data_shared"
		if item%2 == 0 {
			key = "item_data_unique"
		}

		if _, err := cache.Get(ctx, key); err == nil {
			return "hit", nil // cache hit
		}

		val, err := retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
			var fetched string
			execErr := cb.ExecuteContext(ctx, func() error {
				endpoint := srv.URL + "/process?id=ok"
				if item%10 == 0 {
					endpoint = srv.URL + "/process?id=crash"
				}
				req, reqErr := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
				if reqErr != nil {
					return reqErr
				}
				resp, doErr := srv.Client().Do(req)
				if doErr != nil {
					return doErr
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return errors.New("bad status")
				}
				fetched = "fetched"
				return nil
			})
			return fetched, execErr
		}, retry.WithMaxAttempts(2), retry.WithInitialDelay(2*time.Millisecond))

		if err == nil {
			_ = cache.Set(ctx, key, []byte(val), 2*time.Second)
			return val, nil
		}

		return "", err
	})

	// 4. Verify results
	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "bad status") &&
			!strings.Contains(errMsg, "circuit is open") &&
			!strings.Contains(errMsg, "too many requests") &&
			!strings.Contains(errMsg, "max attempts reached") &&
			!errors.Is(err, context.DeadlineExceeded) &&
			!errors.Is(err, context.Canceled) {
			t.Errorf("unexpected catastrophic error: %v", err)
		}
	} else if len(resList) != numItems {
		t.Fatalf("expected %d results, got %d", numItems, len(resList))
	}
}
