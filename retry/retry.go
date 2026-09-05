// Package retry provides backoff and retry mechanisms for transient failures.
// Purpose: retry provides backoff and retry mechanisms for transient failures.
// Constraints: Internal package.
// Thread-safety: Varies by component.
package retry

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"math"
	"math/big"
	"time"
)

// randReader is an internal override point for tests.
// Purpose: Allows injecting mocked random number generators during unit tests.
// Constraints: Must implement io.Reader.
// Thread-safety: Not thread-safe for reassignment, must be mocked before concurrent tests.
var randReader io.Reader = rand.Reader

// ErrMaxAttemptsReached signifies a terminal failure state triggered internally whenever a supervised retry execution loop completely exhausts its configured maximum iteration count without a single success.
// Purpose: Sentinel error indicating total failure of the retry loop.
// Constraints: Can be used with errors.Is.
// Thread-safety: Pure error sentinel, safe for concurrent use.
// Internal Logic Deep-Dive: Distinctly indicates that the underlying service is dead, not just flapping.
var ErrMaxAttemptsReached = errors.New("retry: max attempts reached")

// Strategy isolates the algorithmic methodology leveraged by the delay calculator engine to dictate the exact duration applied during backoff sleep cycles between successive failures.
// Purpose: Used to switch mathematical decay mechanisms.
// Constraints: Expected to be one of the pre-defined constants.
// Thread-safety: Pure enum.
// Internal Logic Deep-Dive: Essential for preventing coordinated retry storms against recovering backend services.
type Strategy int

const (
	// StrategyConstant uses a fixed delay between retries.
	// Purpose: Provides stable looping gaps without degradation.
	// Constraints: Best for predictable internal operations.
	// Thread-safety: Constant value.
	// Internal Logic Deep-Dive: Best used for polling local processes, terrible for network calls.
	StrategyConstant Strategy = iota

	// StrategyExponential uses exponential backoff between retries.
	// Purpose: Helps gracefully de-escalate resource usage while down.
	// Constraints: Recommended for remote network calls.
	// Thread-safety: Constant value.
	// Internal Logic Deep-Dive: Ensures the system organically backs off, allowing databases time to recover from locked states.
	StrategyExponential
)

// Config solidifies the exhaustive behavioral blueprint constraining how aggressively and how frequently a given failure handler attempts to repeat an aborted logical execution block.
// Purpose: Parameterizes retry loops.
// Constraints: Must be populated with sensible bounds.
// Thread-safety: Modifying after initiation is not advised; fields should be considered read-only by runners.
// Internal Logic Deep-Dive: Isolates the arithmetic variables required to calculate complex jitter and backoffs.
type Config struct {
	// MaxAttempts limits the total number of execution iterations before completely failing.
	// Purpose: Provides a hard upper bound on retry loops.
	// Constraints: Should be > 0.
	// Thread-safety: Read-only integer.
	MaxAttempts int
	// InitialDelay is the starting sleep duration used by backoff strategies after the first failure.
	// Purpose: Determines the baseline sleep time.
	// Constraints: Should be >= 0.
	// Thread-safety: Read-only duration.
	InitialDelay time.Duration
	// MaxDelay aggressively caps the exponentially growing sleep duration to prevent infinitely increasing wait times.
	// Purpose: Limits how long a single backoff sleep can take.
	// Constraints: Should be >= InitialDelay.
	// Thread-safety: Read-only duration.
	MaxDelay time.Duration
	// Strategy determines the mathematical decay mechanism applied between subsequent retries.
	// Purpose: Chooses the backoff algorithm (e.g. constant vs exponential).
	// Constraints: Should be a valid Strategy enum.
	// Thread-safety: Read-only enum.
	Strategy Strategy
	// Jitter enables the injection of cryptographic randomness into sleep durations to thwart thundering herd bottlenecks.
	// Purpose: Prevents synchronized retries from overwhelming dependencies.
	// Constraints: Boolean flag.
	// Thread-safety: Read-only boolean.
	Jitter bool
	// RetryIf acts as a custom predicate interceptor, allowing the system to selectively abort retries for non-recoverable errors.
	// Purpose: Allows short-circuiting the retry loop for fatal errors.
	// Constraints: If nil, all errors are retried.
	// Thread-safety: Executed synchronously by the caller goroutine.
	RetryIf func(error) bool
}

// Option establishes a localized mutator function signature, cleanly encapsulating specific parameter overrides targeted at customizing the resilient retry configuration map.
// Purpose: Allows overriding default retry configuration settings.
// Constraints: Apply synchronously before launching the loop.
// Thread-safety: Safe when used sequentially during initialization.
// Internal Logic Deep-Dive: Standardizes initialization patterns.
type Option func(*Config)

// defaultConfig returns sensible default configuration
// that applies safe bounded limits and an exponential strategy.
// Purpose: Generates an internal baseline default configuration for retries.
// Constraints: Should be considered read-only after being returned unless mutated by functional options.
// Thread-safety: Returns a new struct pointer, safe across goroutines before sharing.
func defaultConfig() Config {
	// These default tunings enforce an exponential backoff capped at 10 seconds with full jitter.
	// This specific combination guarantees that widespread transient failures don't
	// accidentally synchronize massive waves of retries that could crush external services.
	return Config{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Strategy:     StrategyExponential,
		Jitter:       true,
		RetryIf:      func(_ error) bool { return true },
	}
}

// WithMaxAttempts directly restricts the absolute ceiling on how many iterative function loops are allowed to evaluate before immediately triggering a catastrophic timeout collapse.
// Purpose: Bound the maximum amount of loops the retry block executes.
// Constraints: Must be positive or default bound is used.
// Thread-safety: Mutates configuration synchronously.
// Internal Logic Deep-Dive: Ensures execution paths eventually yield an error rather than hanging forever.
func WithMaxAttempts(n int) Option {
	return func(c *Config) {
		if n > 0 {
			c.MaxAttempts = n
		}
	}
}

// WithInitialDelay dictates the fundamental wait block assigned following the very first algorithmic failure, functioning as the foundational seed multiplier for more complex decay algorithms.
// Purpose: Define a base wait offset.
// Constraints: Usually bounded by MaxDelay.
// Thread-safety: Mutates configuration synchronously.
// Internal Logic Deep-Dive: Setting a deterministic lower-bound effectively seeds the exponential degradation strategy to guarantee immediate downstream relief during transient disruptions.
func WithInitialDelay(d time.Duration) Option {
	return func(c *Config) {
		c.InitialDelay = d
	}
}

// WithMaxDelay places a strict, impenetrable limit on maximum backoff lengths to prevent escalating mathematical multipliers from forcing goroutines into functionally infinite sleep states.
// Purpose: Limit how far an exponential strategy scales the wait time.
// Constraints: Should be longer than initial delay.
// Thread-safety: Mutates configuration synchronously.
// Internal Logic Deep-Dive: Critically truncates the backoff curve to maintain reasonable API response times.
func WithMaxDelay(d time.Duration) Option {
	return func(c *Config) {
		c.MaxDelay = d
	}
}

// WithStrategy configures the backoff algorithm used to calculate delay intervals between retry attempts.
// Purpose: Configures which algorithm dictates backoff timing.
// Constraints: Assumes StrategyConstant or StrategyExponential.
// Thread-safety: Mutates configuration synchronously.
// Internal Logic Deep-Dive: Plugs in the arithmetic curve engine.
func WithStrategy(s Strategy) Option {
	return func(c *Config) {
		c.Strategy = s
	}
}

// WithJitter flips a boolean trigger that instructs the delay calculator to selectively inject cryptographically secure noise into its math boundaries to circumvent synchronized thundering herd storms.
// Purpose: Avoid concurrent 'thundering herd' spikes after transient outages.
// Constraints: Usually implemented as full random jitter.
// Thread-safety: Mutates configuration synchronously.
// Internal Logic Deep-Dive: Desynchronizes multiple parallel workers that hit the same network hiccup simultaneously.
func WithJitter(enabled bool) Option {
	return func(c *Config) {
		c.Jitter = enabled
	}
}

// WithRetryIf assigns an intelligent discriminator function callback tasked with dissecting incoming application errors, authorizing immediate termination logic for non-transient, permanently unrecoverable problems.
// Purpose: Allows selective short-circuiting for fatal, unrecoverable errors.
// Constraints: If the predicate returns false, the retry loop stops immediately.
// Thread-safety: Mutates configuration synchronously.
// Internal Logic Deep-Dive: Dynamically inspects the error tree, allowing the caller to short-circuit the loop immediately.
func WithRetryIf(fn func(error) bool) Option {
	return func(c *Config) {
		c.RetryIf = fn
	}
}

// Do isolates a purely side-effecting code block, endlessly attempting to push it toward success until artificially restrained by context deadlines, maximum bounds, or fatal error triggers.
// Purpose: Safely wrap side-effect operations in a resilient loop.
// Constraints: It applies the configured backoff strategy between attempts.
// Thread-safety: Safe for concurrent execution, maintaining local state loop
// variables per individual invocation.
// Internal Logic Deep-Dive: The backoff loop utilizes `time.Timer` rather than `time.Sleep` to ensure it can be safely interrupted by a context cancellation, preventing goroutine leaks during prolonged retries.
func Do(ctx context.Context, fn func(ctx context.Context) error, opts ...Option) error {
	if len(opts) == 0 {
		return doWithConfig(ctx, fn, defaultConfig())
	}

	cfg := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return doWithConfig(ctx, fn, cfg)
}

// doWithConfig executes the retry loop using a pre-calculated Config struct.
// Purpose: Contains the core retry engine logic separated from option parsing.
// Constraints: Requires a fully populated Config struct to operate correctly.
// Thread-safety: Operates safely in concurrent contexts; relies on external state immutability.
// Internal Logic: Keeping cfg as a value parameter prevents it from escaping to the heap
// when no options are dynamically evaluated.
func doWithConfig(ctx context.Context, fn func(ctx context.Context) error, cfg Config) error {
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		// Respect the caller's context lifecycle strictly by intercepting cancellations
		// before doing any work, saving network/CPU cycles on aborted processes.
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		// Delegate error introspection to the caller's defined RetryIf condition,
		// allowing immediate bail-outs for deterministic, unrecoverable errors like 404s.
		if !cfg.RetryIf(lastErr) {
			return lastErr
		}

		if attempt < cfg.MaxAttempts-1 {
			delay := calculateDelay(&cfg, attempt)
			// Sleep block multiplexed with the context listener. Ensures that if the context
			// expires during a long exponential backoff sleep, we awake and return instantly.
			// Internal Logic Deep-Dive: We use a `select` block combining `ctx.Done()` with an explicitly instantiated `time.NewTimer(delay)` so that if a massive exponential backoff initiates (e.g., a 10 second sleep), but the incoming request context is suddenly cancelled by the client after 1 second, the goroutine cleanly wakes up and immediately stops the timer (`timer.Stop()`), exiting without needlessly leaking memory or tying up thread resources for the remaining 9 seconds.
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
				timer.Stop()
			}
		}
	}

	// Chain the sentinel error with the underlying error so the caller can inspect
	// the final failure cause using errors.Is or errors.As.
	return errors.Join(ErrMaxAttemptsReached, lastErr)
}

// DoWithValue protects an opaque code segment demanding a concrete return payload in a highly fault-tolerant protective layer, actively resolving intermittent disruptions until retrieving valid data safely.
// Purpose: Safely wrap fallible pure computations or fetches in a resilient loop.
// Constraints: It repeatedly executes fn until it succeeds and returns the result,
// or fails after exhausting all attempts.
// Thread-safety: Safe for concurrent execution, maintaining local state per call.
// Internal Logic Deep-Dive: Internally executes the backoff arithmetic, calculates jitter, and manages timer sleeps while monitoring context cancellation.
func DoWithValue[T any](ctx context.Context, fn func(ctx context.Context) (T, error), opts ...Option) (T, error) {
	if len(opts) == 0 {
		return doWithValueWithConfig(ctx, fn, defaultConfig())
	}

	cfg := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return doWithValueWithConfig(ctx, fn, cfg)
}

// doWithValueWithConfig executes the value-returning retry loop using a pre-calculated Config struct.
// Purpose: Contains the core retry engine logic separated from option parsing, specifically for generic return types.
// Constraints: Requires a fully populated Config struct to operate correctly.
// Thread-safety: Operates safely in concurrent contexts; relies on external state immutability.
// Internal Logic: Keeping cfg as a value parameter prevents it from escaping to the heap
// when no options are dynamically evaluated.
func doWithValueWithConfig[T any](ctx context.Context, fn func(ctx context.Context) (T, error), cfg Config) (T, error) {
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
			delay := calculateDelay(&cfg, attempt)
			// Yield the goroutine to the runtime scheduler securely while monitoring context.
			// Internal Logic Deep-Dive: We use a `select` block combining `ctx.Done()` with an explicitly instantiated `time.NewTimer(delay)` so that if a massive exponential backoff initiates (e.g., a 10 second sleep), but the incoming request context is suddenly cancelled by the client after 1 second, the goroutine cleanly wakes up and immediately stops the timer (`timer.Stop()`), exiting without needlessly leaking memory or tying up thread resources for the remaining 9 seconds.
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return zero, ctx.Err()
			case <-timer.C:
				timer.Stop()
			}
		}
	}
	return zero, errors.Join(ErrMaxAttemptsReached, lastErr)
}

// calculateDelay is an internal helper that computes the exact backoff delay
// for the current attempt based on the chosen strategy.
// Purpose: Applies hard mathematical bounds to prevent extreme sleep times
// and safely injects cryptographic randomness if full jitter is configured.
// Constraints: An internal calculation helper.
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
		jitterVal, err := rand.Int(randReader, big.NewInt(int64(delay)))
		if err == nil {
			delay = time.Duration(jitterVal.Int64())
		}
	}

	return delay
}
