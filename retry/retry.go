// Package retry provides configurable retry logic with exponential backoff,
// jitter, and context-aware cancellation for fallible operations.
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"time"
)

// ErrMaxAttemptsReached is returned when all retry attempts are exhausted.
var ErrMaxAttemptsReached = errors.New("retry: max attempts reached")

// Strategy defines the backoff strategy for retries.
type Strategy int

const (
	// StrategyConstant uses a fixed delay between retries.
	StrategyConstant Strategy = iota
	// StrategyExponential uses exponential backoff between retries.
	StrategyExponential
)

// Config holds the configuration for a retry operation.
type Config struct {
	MaxAttempts int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Strategy     Strategy
	Jitter       bool
	RetryIf      func(error) bool
}

// Option is a functional option for configuring retry behavior.
type Option func(*Config)

// defaultConfig returns sensible default configuration.
func defaultConfig() *Config {
	return &Config{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Strategy:     StrategyExponential,
		Jitter:       true,
		RetryIf:      func(_ error) bool { return true },
	}
}

// WithMaxAttempts sets the maximum number of attempts (including the first).
func WithMaxAttempts(n int) Option {
	return func(c *Config) {
		if n > 0 {
			c.MaxAttempts = n
		}
	}
}

// WithInitialDelay sets the initial delay between retries.
func WithInitialDelay(d time.Duration) Option {
	return func(c *Config) {
		c.InitialDelay = d
	}
}

// WithMaxDelay sets the maximum delay between retries.
func WithMaxDelay(d time.Duration) Option {
	return func(c *Config) {
		c.MaxDelay = d
	}
}

// WithStrategy sets the backoff strategy.
func WithStrategy(s Strategy) Option {
	return func(c *Config) {
		c.Strategy = s
	}
}

// WithJitter enables or disables jitter on the backoff delay.
func WithJitter(enabled bool) Option {
	return func(c *Config) {
		c.Jitter = enabled
	}
}

// WithRetryIf sets a predicate that determines whether an error is retryable.
// If the predicate returns false, the retry loop stops immediately.
func WithRetryIf(fn func(error) bool) Option {
	return func(c *Config) {
		c.RetryIf = fn
	}
}

// Do executes fn, retrying on failure according to the provided options.
// It respects context cancellation and deadlines.
func Do(ctx context.Context, fn func(ctx context.Context) error, opts ...Option) error {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		if !cfg.RetryIf(lastErr) {
			return lastErr
		}

		// Don't sleep after the last attempt.
		if attempt < cfg.MaxAttempts-1 {
			delay := calculateDelay(cfg, attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return errors.Join(ErrMaxAttemptsReached, lastErr)
}

// DoWithValue executes fn which returns a value and an error, retrying on failure.
func DoWithValue[T any](ctx context.Context, fn func(ctx context.Context) (T, error), opts ...Option) (T, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var (
		lastErr error
		zero    T
	)
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		val, err := fn(ctx)
		if err == nil {
			return val, nil
		}
		lastErr = err

		if !cfg.RetryIf(lastErr) {
			return zero, lastErr
		}

		if attempt < cfg.MaxAttempts-1 {
			delay := calculateDelay(cfg, attempt)
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return zero, errors.Join(ErrMaxAttemptsReached, lastErr)
}

// calculateDelay computes the backoff delay for the given attempt.
func calculateDelay(cfg *Config, attempt int) time.Duration {
	var delay time.Duration
	switch cfg.Strategy {
	case StrategyConstant:
		delay = cfg.InitialDelay
	case StrategyExponential:
		multiplier := math.Pow(2, float64(attempt))
		delay = time.Duration(float64(cfg.InitialDelay) * multiplier)
	default:
		delay = cfg.InitialDelay
	}

	if delay > cfg.MaxDelay {
		delay = cfg.MaxDelay
	}

	if cfg.Jitter && delay > 0 {
		// Full jitter: random value between 0 and delay.
		delay = time.Duration(rand.Int64N(int64(delay)))
	}

	return delay
}
