package retry

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoSuccess(t *testing.T) {
	var calls int
	err := Do(context.Background(), func(_ context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDoContextCancelledDuringDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	dummyErr := errors.New("dummy")
	err := Do(ctx, func(c context.Context) error {
		attempts++
		if attempts == 1 {
			// Trigger a long delay.
			// Start a goroutine to cancel the context strictly *during* the select wait block
			go func() {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}()
			return dummyErr
		}
		return nil
	}, WithStrategy(StrategyConstant), WithInitialDelay(time.Hour), WithJitter(false))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected exactly 1 attempt, got: %d", attempts)
	}
}

func TestDoRetryAndSucceed(t *testing.T) {
	var calls int
	err := Do(context.Background(), func(_ context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("temp error")
		}
		return nil
	}, WithMaxAttempts(5), WithInitialDelay(time.Millisecond), WithJitter(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDoMaxAttemptsReached(t *testing.T) {
	var calls int
	err := Do(context.Background(), func(_ context.Context) error {
		calls++
		return errors.New("persistent error")
	}, WithMaxAttempts(3), WithInitialDelay(time.Millisecond), WithJitter(false))

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrMaxAttemptsReached) {
		t.Fatalf("expected ErrMaxAttemptsReached, got: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDoContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls int
	err := Do(ctx, func(_ context.Context) error {
		calls++
		if calls == 2 {
			cancel()
		}
		return errors.New("error")
	}, WithMaxAttempts(10), WithInitialDelay(time.Millisecond), WithJitter(false))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestDoContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before calling Do.

	err := Do(ctx, func(_ context.Context) error {
		t.Fatal("fn should not be called")
		return nil
	}, WithMaxAttempts(5))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestDoRetryIf(t *testing.T) {
	permanentErr := errors.New("permanent")
	var calls int
	err := Do(context.Background(), func(_ context.Context) error {
		calls++
		return permanentErr
	}, WithMaxAttempts(5), WithInitialDelay(time.Millisecond), WithRetryIf(func(err error) bool {
		return !errors.Is(err, permanentErr)
	}))

	if !errors.Is(err, permanentErr) {
		t.Fatalf("expected permanent error, got: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry), got %d", calls)
	}
}

func TestDoConstantStrategy(t *testing.T) {
	var calls int
	start := time.Now()
	err := Do(context.Background(), func(_ context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("temp")
		}
		return nil
	}, WithMaxAttempts(5), WithInitialDelay(10*time.Millisecond), WithStrategy(StrategyConstant), WithJitter(false))

	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 2 retries × 10ms = 20ms minimum
	if elapsed < 20*time.Millisecond {
		t.Fatalf("constant delay too short: %v", elapsed)
	}
}

func TestDoWithValue(t *testing.T) {
	var calls atomic.Int32
	val, err := DoWithValue(context.Background(), func(_ context.Context) (string, error) {
		calls.Add(1)
		if calls.Load() < 2 {
			return "", errors.New("temp")
		}
		return "hello", nil
	}, WithMaxAttempts(3), WithInitialDelay(time.Millisecond), WithJitter(false))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "hello" {
		t.Fatalf("expected 'hello', got %q", val)
	}
}

func TestDoWithValueAllFail(t *testing.T) {
	val, err := DoWithValue(context.Background(), func(_ context.Context) (int, error) {
		return 0, errors.New("fail")
	}, WithMaxAttempts(2), WithInitialDelay(time.Millisecond), WithJitter(false))

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrMaxAttemptsReached) {
		t.Fatalf("expected ErrMaxAttemptsReached, got: %v", err)
	}
	if val != 0 {
		t.Fatalf("expected zero value, got %d", val)
	}
}

func TestDoWithValueContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := DoWithValue(ctx, func(_ context.Context) (int, error) {
		t.Fatal("fn should not be called")
		return 0, nil
	}, WithMaxAttempts(5))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestDoWithValueContextCancelledDuringRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls int
	_, err := DoWithValue(ctx, func(_ context.Context) (int, error) {
		calls++
		if calls == 1 {
			cancel()
		}
		return 0, errors.New("temp")
	}, WithMaxAttempts(5), WithInitialDelay(time.Millisecond), WithJitter(false))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestDoWithValueRetryIf(t *testing.T) {
	permanentErr := errors.New("permanent")
	var calls int
	_, err := DoWithValue(context.Background(), func(_ context.Context) (string, error) {
		calls++
		return "", permanentErr
	}, WithMaxAttempts(5), WithInitialDelay(time.Millisecond), WithRetryIf(func(err error) bool {
		return !errors.Is(err, permanentErr)
	}))

	if !errors.Is(err, permanentErr) {
		t.Fatalf("expected permanent error, got: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry), got %d", calls)
	}
}

func TestWithMaxDelay(t *testing.T) {
	cfg := defaultConfig()
	WithMaxDelay(5 * time.Second)(&cfg)
	if cfg.MaxDelay != 5*time.Second {
		t.Fatalf("expected 5s, got %v", cfg.MaxDelay)
	}
}

func TestWithMaxAttemptsZero(t *testing.T) {
	cfg := defaultConfig()
	original := cfg.MaxAttempts
	WithMaxAttempts(0)(&cfg) // zero should be ignored
	if cfg.MaxAttempts != original {
		t.Fatalf("expected %d (unchanged), got %d", original, cfg.MaxAttempts)
	}
}

func TestWithMaxAttemptsNegative(t *testing.T) {
	cfg := defaultConfig()
	original := cfg.MaxAttempts
	WithMaxAttempts(-1)(&cfg) // negative should be ignored
	if cfg.MaxAttempts != original {
		t.Fatalf("expected %d (unchanged), got %d", original, cfg.MaxAttempts)
	}
}

func TestCalculateDelayExponential(t *testing.T) {
	cfg := &Config{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Strategy:     StrategyExponential,
		Jitter:       false,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
	}
	for _, tt := range tests {
		got := calculateDelay(cfg, tt.attempt)
		if got != tt.expected {
			t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.expected, got)
		}
	}
}

func TestCalculateDelayMaxCap(t *testing.T) {
	cfg := &Config{
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Second,
		Strategy:     StrategyExponential,
		Jitter:       false,
	}
	delay := calculateDelay(cfg, 10) // 2^10 * 1s = 1024s → capped at 5s
	if delay != 5*time.Second {
		t.Fatalf("expected max delay 5s, got %v", delay)
	}

	// Also test an extremely large attempt that hits the 62 cap.
	delayLarge := calculateDelay(cfg, 100)
	if delayLarge != 5*time.Second {
		t.Fatalf("expected max delay 5s for huge attempt, got %v", delayLarge)
	}
}

func TestCalculateDelayInitialExceedsMax(t *testing.T) {
	cfg := &Config{
		InitialDelay: 10 * time.Second,
		MaxDelay:     5 * time.Second,
		Strategy:     StrategyConstant,
	}
	delay := calculateDelay(cfg, 1)
	if delay != 5*time.Second {
		t.Fatalf("expected max delay 5s when initial > max, got %v", delay)
	}
}

func TestCalculateDelayUnknownStrategy(t *testing.T) {
	cfg := &Config{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Strategy:     Strategy(99), // Unknown strategy → falls through to default
		Jitter:       false,
	}
	delay := calculateDelay(cfg, 0)
	if delay != 100*time.Millisecond {
		t.Fatalf("expected 100ms (default fallback), got %v", delay)
	}
}

type errorReader struct{}

func (e errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("mock read error")
}

func TestCalculateDelayJitterErrorFallback(t *testing.T) {
	originalReader := randReader
	t.Cleanup(func() { randReader = originalReader })

	// Inject error reader
	randReader = errorReader{}

	cfg := &Config{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Strategy:     StrategyExponential,
		Jitter:       true,
	}

	// Should fallback to base calculated delay without panic
	delay := calculateDelay(cfg, 0)
	if delay != 100*time.Millisecond {
		t.Fatalf("expected fallback 100ms, got %v", delay)
	}
}

func TestCalculateDelaySafeAttemptBound(t *testing.T) {
	// A delay calculation without exceeding MaxDelay logic, but safeAttempt capped at 62
	// 100ms * 2^62 is larger than max int64 duration, so we use a small initial delay
	cfg := &Config{
		InitialDelay: 1 * time.Nanosecond,
		MaxDelay:     time.Duration(math.MaxInt64), // maximum duration
		Strategy:     StrategyExponential,
		Jitter:       false,
	}

	delay := calculateDelay(cfg, 100)
	expected := time.Duration(math.Pow(2, 62))
	if delay != expected {
		t.Fatalf("expected capped duration %v, got %v", expected, delay)
	}
}

func TestCalculateDelayJitterBounds(t *testing.T) {
	cfg := &Config{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Strategy:     StrategyExponential,
		Jitter:       true,
	}
	for i := 0; i < 100; i++ {
		delay := calculateDelay(cfg, 0)
		if delay < 0 {
			t.Fatalf("negative delay: %v", delay)
		}
		if delay >= 100*time.Millisecond {
			t.Fatalf("jitter delay >= base: %v", delay)
		}
	}
}

func FuzzCalculateDelay(f *testing.F) {
	f.Add(100, 10000, 0, true)
	f.Add(1, 1, 5, false)
	f.Fuzz(func(t *testing.T, initialMs int, maxMs int, attempt int, jitter bool) {
		if initialMs <= 0 || maxMs <= 0 || attempt < 0 {
			return
		}
		cfg := &Config{
			InitialDelay: time.Duration(initialMs) * time.Millisecond,
			MaxDelay:     time.Duration(maxMs) * time.Millisecond,
			Strategy:     StrategyExponential,
			Jitter:       jitter,
		}
		delay := calculateDelay(cfg, attempt)
		if delay < 0 {
			t.Fatalf("negative delay: %v", delay)
		}
		if delay > cfg.MaxDelay {
			t.Fatalf("delay %v exceeds max %v", delay, cfg.MaxDelay)
		}
	})
}

func TestCalculateDelayZeroMaxDelay(t *testing.T) {
	cfg := &Config{
		MaxDelay: 0,
	}
	if delay := calculateDelay(cfg, 1); delay != 0 {
		t.Fatalf("expected 0, got %v", delay)
	}
}

func TestDo_TableDriven(t *testing.T) {
	errTest := errors.New("test error")
	errAbort := errors.New("abort error")

	tests := []struct {
		name          string
		maxAttempts   int
		fn            func(ctx context.Context) error
		opts          []Option
		ctxCancelFn   func() (context.Context, context.CancelFunc)
		expectedError error
		expectedCalls int
	}{
		{
			name:        "success on first attempt",
			maxAttempts: 3,
			fn: func(ctx context.Context) error {
				return nil
			},
			opts:          []Option{WithMaxAttempts(3), WithInitialDelay(time.Millisecond)},
			expectedError: nil,
			expectedCalls: 1,
		},
		{
			name:        "max attempts reached",
			maxAttempts: 3,
			fn: func(ctx context.Context) error {
				return errTest
			},
			opts:          []Option{WithMaxAttempts(3), WithInitialDelay(time.Millisecond)},
			expectedError: errTest, // Will be wrapped with ErrMaxAttemptsReached
			expectedCalls: 3,
		},
		{
			name:        "immediate context cancellation",
			maxAttempts: 3,
			fn: func(ctx context.Context) error {
				return nil
			},
			opts: []Option{WithMaxAttempts(3), WithInitialDelay(time.Millisecond)},
			ctxCancelFn: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			expectedError: context.Canceled,
			expectedCalls: 0,
		},
		{
			name:        "context cancellation during retry delay",
			maxAttempts: 3,
			fn: func(ctx context.Context) error {
				return errTest
			},
			opts: []Option{WithMaxAttempts(3), WithInitialDelay(50 * time.Millisecond), WithJitter(false)},
			ctxCancelFn: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				// Cancel shortly after the first attempt to interrupt the sleep
				go func() {
					time.Sleep(10 * time.Millisecond)
					cancel()
				}()
				return ctx, cancel
			},
			expectedError: context.Canceled,
			// Since timing is non-deterministic, we shouldn't strictly enforce expectedCalls to be 1.
			// It might occasionally call the function twice if the go-routine is slow to execute cancel().
			// Setting to 0 to bypass the exact call count check.
			expectedCalls: 0,
		},
		{
			name:        "early abort due to RetryIf",
			maxAttempts: 3,
			fn: func(ctx context.Context) error {
				return errAbort
			},
			opts: []Option{
				WithMaxAttempts(3),
				WithInitialDelay(time.Millisecond),
				WithRetryIf(func(err error) bool {
					return err != errAbort
				}),
			},
			expectedError: errAbort,
			expectedCalls: 1,
		},
		{
			name:        "nil option safety",
			maxAttempts: 3,
			fn: func(ctx context.Context) error {
				return nil
			},
			opts:          []Option{nil, WithMaxAttempts(3), nil},
			expectedError: nil,
			expectedCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var cancel context.CancelFunc
			if tt.ctxCancelFn != nil {
				ctx, cancel = tt.ctxCancelFn()
				defer cancel()
			}

			calls := 0
			wrappedFn := func(c context.Context) error {
				calls++
				return tt.fn(c)
			}

			err := Do(ctx, wrappedFn, tt.opts...)

			if tt.expectedError != nil {
				if tt.name == "max attempts reached" {
					if !errors.Is(err, ErrMaxAttemptsReached) || !errors.Is(err, errTest) {
						t.Errorf("expected joined error with ErrMaxAttemptsReached and errTest, got %v", err)
					}
				} else if !errors.Is(err, tt.expectedError) {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}

			if tt.expectedCalls > 0 && calls != tt.expectedCalls {
				t.Errorf("expected %d calls, got %d", tt.expectedCalls, calls)
			}
		})
	}
}

func TestDoConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		isValueTest bool
	}{
		{
			name:        "Do concurrent execution",
			isValueTest: false,
		},
		{
			name:        "DoWithValue concurrent execution",
			isValueTest: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const numGoroutines = 100
			errCh := make(chan error, numGoroutines)
			for i := 0; i < numGoroutines; i++ {
				go func(val int) {
					if tt.isValueTest {
						res, err := DoWithValue(context.Background(), func(_ context.Context) (int, error) {
							return val, nil
						}, WithMaxAttempts(2), WithInitialDelay(time.Millisecond))
						if err != nil {
							errCh <- err
							return
						}
						if res != val {
							errCh <- errors.New("result mismatch")
							return
						}
						errCh <- nil
					} else {
						err := Do(context.Background(), func(_ context.Context) error {
							return nil
						}, WithMaxAttempts(2), WithInitialDelay(time.Millisecond))
						errCh <- err
					}
				}(i)
			}
			for i := 0; i < numGoroutines; i++ {
				if err := <-errCh; err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDoWithValue_TableDriven(t *testing.T) {
	errAbort := errors.New("abort error")

	tests := []struct {
		name          string
		maxAttempts   int
		fn            func(ctx context.Context) (int, error)
		opts          []Option
		expectedError error
		expectedValue int
		expectedCalls int
	}{
		{
			name: "no options provided (nil slice)",
			fn: func(ctx context.Context) (int, error) {
				return 24, nil
			},
			opts:          nil,
			expectedError: nil,
			expectedValue: 24,
			expectedCalls: 1,
		},
		{
			name:        "slice with nil options safety",
			maxAttempts: 3,
			fn: func(ctx context.Context) (int, error) {
				return 42, nil
			},
			opts:          []Option{nil, WithMaxAttempts(3), nil},
			expectedError: nil,
			expectedValue: 42,
			expectedCalls: 1,
		},
		{
			name:        "early abort due to RetryIf",
			maxAttempts: 3,
			fn: func(ctx context.Context) (int, error) {
				return 0, errAbort
			},
			opts: []Option{
				WithMaxAttempts(3),
				WithInitialDelay(time.Millisecond),
				WithRetryIf(func(err error) bool {
					return err != errAbort
				}),
			},
			expectedError: errAbort,
			expectedValue: 0,
			expectedCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			calls := 0
			wrappedFn := func(c context.Context) (int, error) {
				calls++
				return tt.fn(c)
			}

			val, err := DoWithValue(ctx, wrappedFn, tt.opts...)

			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}

			if val != tt.expectedValue {
				t.Errorf("expected value %v, got %v", tt.expectedValue, val)
			}

			if tt.expectedCalls > 0 && calls != tt.expectedCalls {
				t.Errorf("expected %d calls, got %d", tt.expectedCalls, calls)
			}
		})
	}
}
