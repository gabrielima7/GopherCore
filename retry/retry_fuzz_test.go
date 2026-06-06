package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

// FuzzRetryDo validates the bounds processing and timing safety of the Retry engine by throwing unpredictable, potentially hostile configuration boundaries at the calculation layer.
func FuzzRetryDo(f *testing.F) {
	// Seed realistic configurations
	f.Add(3, 100, 1000, 10, true)
	f.Add(10, 10, 50, 20, false)
	f.Add(1, 0, 0, 5, true)

	f.Fuzz(func(t *testing.T, maxAttempts int, initialDelayMs int, maxDelayMs int, simulatedFailures int, jitter bool) {
		// Enforce test boundary constraints to prevent infinite freezes
		if maxAttempts <= 0 {
			maxAttempts = 1
		}
		if maxAttempts > 100 {
			maxAttempts = 100
		}
		if initialDelayMs < 0 {
			initialDelayMs = 0
		}
		if initialDelayMs > 1000 {
			initialDelayMs = 1000
		}
		if maxDelayMs < initialDelayMs {
			maxDelayMs = initialDelayMs
		}
		if maxDelayMs > 5000 {
			maxDelayMs = 5000
		}
		if simulatedFailures < 0 {
			simulatedFailures = 0
		}

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		failures := 0
		errTest := errors.New("fuzz transient error")

		_ = Do(ctx, func(c context.Context) error {
			if failures < simulatedFailures {
				failures++
				return errTest
			}
			return nil
		},
			WithMaxAttempts(maxAttempts),
			WithInitialDelay(time.Duration(initialDelayMs)*time.Millisecond),
			WithMaxDelay(time.Duration(maxDelayMs)*time.Millisecond),
			WithJitter(jitter),
			WithStrategy(StrategyExponential),
		)
	})
}
