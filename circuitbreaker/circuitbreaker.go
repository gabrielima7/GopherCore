// Package circuitbreaker implements the Circuit Breaker pattern to prevent
// cascading failures. It wraps fallible operations and trips when too many
// failures occur, allowing the system to recover gracefully.
package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit is in the Open state
// and no requests are permitted to execute. Callers should fast-fail
// or fallback to a secondary mechanism.
var ErrCircuitOpen = errors.New("circuitbreaker: circuit is open")

// ErrTooManyRequests is returned when the circuit is in the HalfOpen
// state and the maximum number of concurrent probe requests has already
// been reached.
var ErrTooManyRequests = errors.New("circuitbreaker: too many requests in half-open state")

// State represents the current operational state of the circuit breaker.
type State int

const (
	// StateClosed is the normal operational state. All requests are allowed
	// through. The breaker counts consecutive failures to determine if it
	// should trip to StateOpen.
	StateClosed State = iota
	// StateOpen is the tripped state. All requests are immediately rejected
	// with ErrCircuitOpen until the configured timeout duration expires.
	StateOpen
	// StateHalfOpen is the recovery state. A limited number of probe requests
	// are allowed through to test if the underlying service has recovered.
	StateHalfOpen
)

// String returns the human-readable string representation of the State.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config holds the configuration parameters that dictate the behavior
// and thresholds of a circuit breaker.
type Config struct {
	// FailureThreshold is the number of consecutive failures before
	// the circuit breaker transitions from Closed to Open.
	FailureThreshold int

	// SuccessThreshold is the number of consecutive successes in
	// HalfOpen state required to transition back to Closed.
	SuccessThreshold int

	// Timeout is the duration the circuit stays in the Open state
	// before transitioning to HalfOpen.
	Timeout time.Duration

	// MaxHalfOpenRequests is the maximum number of requests allowed
	// in the HalfOpen state. Defaults to 1.
	MaxHalfOpenRequests int

	// OnStateChange is called when the circuit breaker transitions state.
	OnStateChange func(from, to State)
}

// DefaultConfig returns a sensible default configuration for a circuit breaker:
// 5 failures to open, 30 seconds open timeout, 1 half-open request, and 2
// consecutive successes to close.
func DefaultConfig() Config {
	return Config{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		MaxHalfOpenRequests: 1,
	}
}

// Breaker is a thread-safe implementation of the Circuit Breaker pattern.
// It coordinates concurrent access to the circuit's state and statistics.
type Breaker struct {
	mu     sync.Mutex
	config Config

	state            State
	failureCount     int
	successCount     int
	halfOpenRequests int
	lastFailureTime  time.Time
}

// New creates and returns a new Breaker initialized with the given configuration.
// It will apply sensible default values for any configuration fields that are
// left as zero or invalid (<= 0). The breaker starts in the StateClosed state.
func New(cfg Config) *Breaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxHalfOpenRequests <= 0 {
		cfg.MaxHalfOpenRequests = 1
	}
	return &Breaker{config: cfg, state: StateClosed}
}

// State safely retrieves and returns the current operational state of the circuit breaker.
// It handles potential state transitions (e.g., from Open to HalfOpen) if the timeout
// has expired before returning the state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentState()
}

// Execute executes the provided function fn if the circuit breaker determines
// that requests are currently permitted.
// If the circuit is Open, it immediately returns ErrCircuitOpen.
// If the circuit is HalfOpen and the maximum probe limit is exceeded, it returns ErrTooManyRequests.
// Otherwise, it runs fn, records the success or failure of the execution to
// update internal statistics, and returns the error produced by fn.
// It is fully safe for concurrent use across multiple goroutines, executing
// the fallback logic within internal mutex locks to ensure accurate state tracking.
func (b *Breaker) Execute(fn func() error) error {
	b.mu.Lock()

	state := b.currentState()

	switch state {
	case StateOpen:
		b.mu.Unlock()
		// Circuit is open, immediately fast-fail the request to prevent
		// further strain on the failing underlying service.
		return ErrCircuitOpen
	case StateHalfOpen:
		// Limit the number of concurrent probe requests in HalfOpen state
		// to test recovery without overwhelming the service.
		if b.halfOpenRequests >= b.config.MaxHalfOpenRequests {
			b.mu.Unlock()
			return ErrTooManyRequests
		}
		b.halfOpenRequests++
	}

	b.mu.Unlock()

	// Execute the user-provided function outside the lock to ensure
	// the lock is not held during potentially long-running I/O operations.
	err := fn()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Update the circuit breaker statistics based on the execution result.
	if err != nil {
		b.recordFailure()
	} else {
		b.recordSuccess()
	}

	return err
}

// currentState evaluates and returns the current state.
// If the state is Open, it checks if the timeout duration has elapsed
// since the last failure. If so, it automatically transitions the state
// to HalfOpen to allow probe requests.
// This function requires the Breaker's mutex to be held by the caller.
func (b *Breaker) currentState() State {
	// Check if the circuit should automatically transition from Open to HalfOpen.
	if b.state == StateOpen {
		if time.Since(b.lastFailureTime) >= b.config.Timeout {
			b.transitionTo(StateHalfOpen)
		}
	}
	return b.state
}

// recordSuccess updates internal statistics following a successful execution.
// In the Closed state, it resets the consecutive failure count.
// In the HalfOpen state, it increments the success count and transitions
// back to Closed if the success threshold is met.
// This function requires the Breaker's mutex to be held by the caller.
func (b *Breaker) recordSuccess() {
	switch b.state {
	case StateClosed:
		b.failureCount = 0
	case StateHalfOpen:
		b.successCount++
		if b.successCount >= b.config.SuccessThreshold {
			b.transitionTo(StateClosed)
		}
	}
}

// recordFailure updates internal statistics following a failed execution.
// It records the time of the failure. In the Closed state, it increments the
// failure count and transitions to Open if the threshold is reached.
// In the HalfOpen state, any failure immediately trips the circuit back to Open.
// This function requires the Breaker's mutex to be held by the caller.
func (b *Breaker) recordFailure() {
	b.lastFailureTime = time.Now()

	switch b.state {
	case StateClosed:
		b.failureCount++
		if b.failureCount >= b.config.FailureThreshold {
			b.transitionTo(StateOpen)
		}
	case StateHalfOpen:
		b.transitionTo(StateOpen)
	}
}

// transitionTo safely changes the circuit breaker's state to newState,
// resetting all internal tracking counters (failures, successes, half-open requests).
// If a state change callback is configured, it is invoked synchronously.
// This function requires the Breaker's mutex to be held by the caller.
func (b *Breaker) transitionTo(newState State) {
	if b.state == newState {
		return
	}
	from := b.state
	b.state = newState
	b.failureCount = 0
	b.successCount = 0
	b.halfOpenRequests = 0

	if b.config.OnStateChange != nil {
		b.config.OnStateChange(from, to(newState))
	}
}

// to is an internal helper that simply returns the provided State value.
// It is used to bypass variable shadowing issues in closure contexts.
func to(s State) State { return s }

// Reset forcefully resets the circuit breaker back to the normal Closed state,
// regardless of its current state or failure statistics.
// This safely locks the internal mutex to prevent race conditions during reset.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transitionTo(StateClosed)
}
