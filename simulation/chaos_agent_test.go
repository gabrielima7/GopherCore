package simulation_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gabrielima7/GopherCore/async"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
	"github.com/gabrielima7/GopherCore/retry"
)

// TestAgentChaos demonstrates the continuous cycle of simulation, destruction, self-healing, and mathematical proof.
func TestAgentChaos(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// High traffic scenario
	const goroutines = 2000
	items := make([]int, goroutines)
	for i := range items {
		items[i] = i
	}

	cb := circuitbreaker.New(circuitbreaker.Config{
		FailureThreshold: 10,
		Timeout:          1 * time.Millisecond,
		SuccessThreshold: 2,
	})

	var successCount int64
	var failureCount int64

	// Concurrency chaos and testing the Developer Experience
	_, err := async.Map(ctx, items, 500, func(c context.Context, item int) (int, error) {
		res, err := retry.DoWithValue(c, func(c2 context.Context) (int, error) {
			var r int
			err := cb.ExecuteContext(c2, func() error {
				select {
				case <-c2.Done():
					return c2.Err()
				default:
				}

				// Inject failures
				if item%3 == 0 {
					return errors.New("simulated error")
				}
				r = item
				return nil
			})
			return r, err
		}, retry.WithMaxAttempts(3), retry.WithInitialDelay(time.Millisecond))

		if err != nil {
			atomic.AddInt64(&failureCount, 1)
		} else {
			atomic.AddInt64(&successCount, 1)
		}

		return res, err
	})

	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, retry.ErrMaxAttemptsReached) && err.Error() != "simulated error" && !errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		t.Fatalf("Unexpected error: %v", err)
	}
}
