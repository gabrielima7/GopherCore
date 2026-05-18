package simulation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/jsonutil"
	"github.com/gabrielima7/GopherCore/result"
	"github.com/gabrielima7/GopherCore/retry"
)

type Payload struct {
	ID string `json:"id"`
}

func TestChaosMicroserviceSimulation(t *testing.T) {
	// Initialize Circuit Breaker
	cb := circuitbreaker.New(circuitbreaker.DefaultConfig())

	// Initialize Server Router
	router := httpkit.NewRouter(
		httpkit.WithRateLimit(10000, 20000),
		httpkit.WithCORS("*"),
	)

	router.Get("/process", func(w http.ResponseWriter, r *http.Request) {
		// Simulate unmarshal of untrusted payload
		var p Payload
		if err := jsonutil.Unmarshal([]byte(`{"id": "test"}`), &p); err != nil {
			httpkit.Error(w, http.StatusBadRequest, "invalid payload")
			return
		}

		// Simulate success
		httpkit.JSON(w, http.StatusOK, map[string]string{"status": "ok", "id": p.ID})
	})

	srv := httptest.NewServer(router)
	defer srv.Close()

	var wg sync.WaitGroup
	startCh := make(chan struct{})

	concurrencyLimit := 1000

	// Inject Concurrency Chaos
	for i := 0; i < concurrencyLimit; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-startCh // Block until start signal to ensure simultaneous execution

			// Simulate context cancellations
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if id%10 == 0 {
				cancel() // Force immediate cancellation for 10% of requests
			}

			// Integrate Retry, CircuitBreaker, Result, and HTTP
			res := result.Of(retry.DoWithValue(ctx, func(ctx context.Context) (string, error) {
				var finalVal string
				err := cb.Execute(func() error {
					req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/process", nil)
					if err != nil {
						return err
					}

					client := &http.Client{}
					resp, err := client.Do(req)
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
			}, retry.WithMaxAttempts(3)))

			_ = res.UnwrapOr("fallback")
		}(i)
	}

	// Test async Group under chaos
	g := async.NewGroup()
	for i := 0; i < 50; i++ {
		g.Go(func() error {
			<-startCh
			return nil
		})
	}

	close(startCh) // Unleash chaos
	wg.Wait()
	_ = g.Wait()
}
