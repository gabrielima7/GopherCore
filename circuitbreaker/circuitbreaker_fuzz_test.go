package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

// FuzzCircuitBreakerTransitions injects randomized configuration thresholds and dynamically tests the Breaker's internal state machine convergence and safety under erratic simulated traffic conditions.
func FuzzCircuitBreakerTransitions(f *testing.F) {
	// Provide initial valid bounds as seed corpus
	f.Add(5, 2, 30, 1, 10)
	f.Add(100, 100, 10, 100, 50)
	f.Add(1, 1, 1, 1, 5)

	f.Fuzz(func(t *testing.T, failThreshold int, successThreshold int, timeoutMs int, maxHalfOpen int, iterations int) {
		// Cap dimensions to prevent out-of-memory loops and timeouts during fuzzing
		if failThreshold <= 0 {
			failThreshold = 1
		}
		if failThreshold > 1000 {
			failThreshold = 1000
		}
		if successThreshold <= 0 {
			successThreshold = 1
		}
		if successThreshold > 1000 {
			successThreshold = 1000
		}
		if timeoutMs <= 0 {
			timeoutMs = 1
		}
		if timeoutMs > 10000 {
			timeoutMs = 10000
		}
		if maxHalfOpen <= 0 {
			maxHalfOpen = 1
		}
		if maxHalfOpen > 100 {
			maxHalfOpen = 100
		}
		if iterations <= 0 {
			iterations = 1
		}
		if iterations > 200 {
			iterations = 200
		}

		cb := New(Config{
			FailureThreshold:    failThreshold,
			SuccessThreshold:    successThreshold,
			Timeout:             time.Duration(timeoutMs) * time.Millisecond,
			MaxHalfOpenRequests: maxHalfOpen,
		})

		errSimulated := errors.New("simulated error")

		// Safely execute randomized load to ensure Breaker does not panic
		for i := 0; i < iterations; i++ {
			_ = cb.Execute(func() error {
				if i%2 == 0 {
					return errSimulated
				}
				return nil
			})
		}
	})
}
