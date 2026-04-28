package retry

import (
	"context"
	"errors"
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
	WithMaxDelay(5 * time.Second)(cfg)
	if cfg.MaxDelay != 5*time.Second {
		t.Fatalf("expected 5s, got %v", cfg.MaxDelay)
	}
}

func TestWithMaxAttemptsZero(t *testing.T) {
	cfg := defaultConfig()
	original := cfg.MaxAttempts
	WithMaxAttempts(0)(cfg) // zero should be ignored
	if cfg.MaxAttempts != original {
		t.Fatalf("expected %d (unchanged), got %d", original, cfg.MaxAttempts)
	}
}

func TestWithMaxAttemptsNegative(t *testing.T) {
	cfg := defaultConfig()
	original := cfg.MaxAttempts
	WithMaxAttempts(-1)(cfg) // negative should be ignored
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
