// Package retry provides configurable retry logic with exponential backoff,
// jitter, and context-aware cancellation for fallible operations.
package retry

import (
	"context"
	"crypto/rand"
	"errors"
	"math"
	"math/big"
	"time"
)

// ErrMaxAttemptsReached is returned when all retry attempts are exhausted.
var ErrMaxAttemptsReached = errors.New("retry: max attempts reached")

// Strategy defines the backoff algorithm used to calculate the delay
// between consecutive retry attempts.
// Thread-safety: Pure enum.
type Strategy int

const (
	// StrategyConstant uses a fixed delay between retries.
	StrategyConstant Strategy = iota
	// StrategyExponential uses exponential backoff between retries.
	StrategyExponential
)

// Config strictly holds the configuration parameters that govern
// the behavior of a retry operation, including backoff algorithms and constraints.
// Thread-safety: Modifying after initiation is not advised; fields should be considered read-only by runners.
type Config struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Strategy     Strategy
	Jitter       bool
	RetryIf      func(error) bool
}

// Option defines a functional option signature for configuring retry behavior
// mutatively during initialization.
type Option func(*Config)

// defaultConfig returns sensible default configuration
// that applies safe bounded limits and an exponential strategy.
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
// Thread-safety: Mutates configuration synchronously.
func WithMaxAttempts(n int) Option {
	return func(c *Config) {
		if n > 0 {
			c.MaxAttempts = n
		}
	}
}

// WithInitialDelay sets the initial delay between retries.
// Thread-safety: Mutates configuration synchronously.
func WithInitialDelay(d time.Duration) Option {
	return func(c *Config) {
		c.InitialDelay = d
	}
}

// WithMaxDelay sets the maximum delay between retries.
// Thread-safety: Mutates configuration synchronously.
func WithMaxDelay(d time.Duration) Option {
	return func(c *Config) {
		c.MaxDelay = d
	}
}

// WithStrategy sets the backoff strategy.
// Thread-safety: Mutates configuration synchronously.
func WithStrategy(s Strategy) Option {
	return func(c *Config) {
		c.Strategy = s
	}
}

// WithJitter enables or disables jitter on the backoff delay.
// Thread-safety: Mutates configuration synchronously.
func WithJitter(enabled bool) Option {
	return func(c *Config) {
		c.Jitter = enabled
	}
}

// WithRetryIf sets a predicate that determines whether an error is retryable.
//
// Constraints: If the predicate returns false, the retry loop stops immediately.
// Thread-safety: Mutates configuration synchronously.
func WithRetryIf(fn func(error) bool) Option {
	return func(c *Config) {
		c.RetryIf = fn
	}
}

// Do repeatedly executes the provided function fn until it succeeds,
// the maximum number of attempts is exhausted, or the context is canceled.
//
// Constraints: It applies the configured backoff strategy between attempts.
// Thread-safety: Safe for concurrent execution, maintaining local state loop
// variables per individual invocation.
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

		// Delay before the next attempt, unless this was the final attempt.
		if attempt < cfg.MaxAttempts-1 {
			delay := calculateDelay(cfg, attempt)
			// Wait for the delay or abort immediately if the context is canceled.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return errors.Join(ErrMaxAttemptsReached, lastErr)
}

// DoWithValue acts identical to Do, but is designed for functions that return
// both a value and an error.
//
// Constraints: It repeatedly executes fn until it succeeds and returns the result,
// or fails after exhausting all attempts.
// Thread-safety: Safe for concurrent execution, maintaining local state per call.
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

// calculateDelay is an internal helper that computes the exact backoff delay
// for the current attempt based on the chosen strategy.
//
// Purpose: Applies hard mathematical bounds to prevent extreme sleep times
// and safely injects cryptographic randomness if full jitter is configured.
// Thread-safety: Relies on `crypto/rand` which handles concurrent random draws safely.
func calculateDelay(cfg *Config, attempt int) time.Duration {
	var delay time.Duration
	switch cfg.Strategy {
	case StrategyConstant:
		delay = cfg.InitialDelay
	case StrategyExponential:
		// Cap attempt at 62 to prevent math.Pow(2, 63) from overflowing float64 -> int64 duration casting
		safeAttempt := attempt
		if safeAttempt > 62 {
			safeAttempt = 62
		}
		multiplier := math.Pow(2, float64(safeAttempt))
		calc := float64(cfg.InitialDelay) * multiplier
		if calc > float64(cfg.MaxDelay) {
			delay = cfg.MaxDelay
		} else {
			delay = time.Duration(calc)
		}
	default:
		delay = cfg.InitialDelay
	}

	if delay > cfg.MaxDelay {
		delay = cfg.MaxDelay
	}

	if cfg.Jitter && delay > 0 {
		// Full jitter: random value between 0 and delay.
		jitterVal, err := rand.Int(rand.Reader, big.NewInt(int64(delay)))
		if err == nil {
			delay = time.Duration(jitterVal.Int64())
		}
	}

	return delay
}
