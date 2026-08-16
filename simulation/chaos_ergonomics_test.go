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
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/logkit"
	"github.com/gabrielima7/GopherCore/result"
	"github.com/gabrielima7/GopherCore/retry"
	"go.uber.org/goleak"
)

func TestErgonomicsAndChaosIntegration(t *testing.T) {
	defer goleak.VerifyNone(t)

	// 1. Evaluate logkit configuration (Ergonomics)
	logkit.Initialize()

	// 2. Evaluate HTTP router with middleware (httpkit)
	router := httpkit.NewRouter(
		httpkit.WithLogger(true),
		httpkit.WithCORS("*"),
	)
	var processed atomic.Int64
	router.Get("/process", func(w http.ResponseWriter, r *http.Request) {
		processed.Add(1)
		if r.URL.Query().Get("fail") == "true" {
			httpkit.Error(w, http.StatusInternalServerError, "simulated chaos failure")
			return
		}
		httpkit.Ok(w, map[string]string{"msg": "ok"})
	})
	httpServer := httptest.NewServer(router)
	defer httpServer.Close()

	// 3. Evaluate Circuit Breaker and Retry setup
	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())

	// 4. Simulate thousands of goroutines accessing the HTTP endpoint with result pattern
	const concurrency = 1000
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var reqs []int
	for i := 0; i < concurrency; i++ {
		reqs = append(reqs, i)
	}

	_, err := async.Map(ctx, reqs, 200, func(c context.Context, id int) (string, error) {
		// Use the Result pattern idiomatically
		res := result.Of(retry.DoWithValue(c, func(ctx context.Context) (string, error) {
			var final string
			cbErr := cb.Execute(func() error {
				endpoint := httpServer.URL + "/process"
				if id%5 == 0 {
					endpoint += "?fail=true"
				}
				req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
				resp, doErr := httpServer.Client().Do(req)
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
		}, retry.WithMaxAttempts(3)))

		if res.IsErr() {
			return "", res.Error()
		}
		val, _ := res.Unwrap()
		return val, nil
	})

	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "bad status") &&
			!strings.Contains(errMsg, "circuit is open") &&
			!strings.Contains(errMsg, "max attempts reached") &&
			!strings.Contains(errMsg, "too many requests") &&
			!errors.Is(err, context.DeadlineExceeded) &&
			!errors.Is(err, context.Canceled) &&
			!errors.Is(err, circuitbreaker.ErrCircuitOpen) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}
