package simulation

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gabrielima7/GopherCore/cachekit"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/httpkit"
	"github.com/gabrielima7/GopherCore/retry"
	"go.uber.org/goleak"
)

func TestGalacticChaosSimulation(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))

	// Setup HTTP Server
	r := httpkit.NewRouter()
	r.Get("/chaos", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	srv := httptest.NewServer(r)
	defer srv.Close()

	client := srv.Client()
	defer client.CloseIdleConnections()

	// Setup Cache
	cache := cachekit.NewInMemoryCache(10 * time.Millisecond)
	defer cache.Close()

	// Setup Circuit Breaker
	cb := circuitbreaker.New(circuitbreaker.Config{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
	})

	var requestCount int64
	concurrency := 15000

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			key := fmt.Sprintf("key-%d", id%10)

			err := cb.ExecuteContext(ctx, func() error {
				return retry.Do(ctx, func(ctx context.Context) error {
					req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/chaos", nil)
					resp, err := client.Do(req)
					if err != nil {
						return err
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("bad status: %d", resp.StatusCode)
					}

					_ = cache.Set(ctx, key, []byte("val"), 5*time.Millisecond)
					_, _ = cache.Get(ctx, key)

					atomic.AddInt64(&requestCount, 1)
					return nil
				}, retry.WithMaxAttempts(3), retry.WithInitialDelay(1*time.Millisecond))
			})

			if err != nil {
				if strings.Contains(err.Error(), "context deadline exceeded") ||
					strings.Contains(err.Error(), "circuit breaker is open") {
					// Expected under extreme load
					_ = err
				}
			}
		}(i)
	}

	wg.Wait()
}
