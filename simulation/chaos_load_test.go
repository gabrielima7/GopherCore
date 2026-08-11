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
	"github.com/gabrielima7/GopherCore/result"
	"github.com/gabrielima7/GopherCore/retry"
	"go.uber.org/goleak"
)

func TestMassiveConcurrencyLoad(t *testing.T) {
	defer goleak.VerifyNone(t)
	// 1. Setup simulated backend
	router := httpkit.NewRouter(
		httpkit.WithRateLimit(100000, 200000), // Huge rate limit
	)
	router.Get("/process", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "crash" {
			httpkit.Error(w, http.StatusInternalServerError, "simulated backend crash")
			return
		}
		httpkit.JSON(w, http.StatusOK, map[string]string{"msg": "success", "id": id})
	})
	srv := httptest.NewServer(router)
	defer srv.Close()

	// 2. Setup internal infrastructure components
	cache := cachekit.NewInMemoryCache(1 * time.Second)
	defer func() { _ = cache.Close() }()

	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const numItems = 20000
	const concurrencyLimit = 500

	items := make([]int, numItems)
	for i := 0; i < numItems; i++ {
		items[i] = i
	}

	// 3. Actively execute bounded concurrency mapping
	resList, err := async.Map(ctx, items, concurrencyLimit, func(ctx context.Context, item int) (string, error) {
		key := "item_data"

		// Optional Context Cancellation Simulation
		if item%500 == 0 {
			var cancelFn context.CancelFunc
			ctx, cancelFn = context.WithCancel(ctx)
			cancelFn() // Immediate cancellation
		}

		if _, err := cache.Get(ctx, key); err == nil {
			return "hit", nil // cache hit
		}

		res := result.Of(retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
			var val string
			err := cb.Execute(func() error {
				endpoint := srv.URL + "/process?id=ok"
				if item%10 == 0 {
					endpoint = srv.URL + "/process?id=crash"
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
				val = "fetched"
				return nil
			})
			return val, err
		}, retry.WithMaxAttempts(2), retry.WithInitialDelay(5*time.Millisecond)))

		if res.IsOk() {
			val, _ := res.Unwrap()
			_ = cache.Set(ctx, key, []byte(val), 2*time.Second)
			return val, nil
		}

		return "", res.Error()
	})

	// 4. Verify properties and mathematical bounds
	// async.Map returns ([]T, error) where error is an aggregated multierror
	if err != nil {
		t.Logf("Map completed with aggregated errors")
	}

	if len(resList) != numItems {
		t.Fatalf("expected %d results, got %d", numItems, len(resList))
	}

	// If there's an error, it's a joinError from errors.Join inside async.Map.
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
			t.Errorf("unexpected catastrophic error: %v", err)
		}
	}
}
