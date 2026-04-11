// Package circuitbreaker implements the Circuit Breaker pattern to prevent
// cascading failures. It wraps fallible operations and trips when too many
// failures occur, allowing the system to recover gracefully.
package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// Sentinel errors returned by the circuit breaker.
var (
	// ErrCircuitOpen is returned when the circuit is in the Open state
	// and no requests are allowed through.
	ErrCircuitOpen = errors.New("circuitbreaker: circuit is open")

	// ErrTooManyRequests is returned when the circuit is in the HalfOpen
	// state and the maximum number of probe requests has been reached.
	ErrTooManyRequests = errors.New("circuitbreaker: too many requests in half-open state")
)

// State represents the current state of the circuit breaker.
type State int

const (
	// StateClosed allows all requests through and counts failures.
	StateClosed State = iota
	// StateOpen blocks all requests and waits for the timeout to expire.
	StateOpen
	// StateHalfOpen allows a limited number of probe requests to test recovery.
	StateHalfOpen
)

// String returns the human-readable name of the state.
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

// Config configures the behavior of a circuit breaker.
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

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		MaxHalfOpenRequests: 1,
	}
}

// Breaker is a thread-safe circuit breaker.
type Breaker struct {
	mu     sync.Mutex
	config Config

	state             State
	failureCount      int
	successCount      int
	halfOpenRequests   int
	lastFailureTime   time.Time
}

// New creates a new Breaker with the given configuration.
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

// State returns the current state of the circuit breaker.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentState()
}

// Execute runs fn if the circuit breaker allows it.
// Returns ErrCircuitOpen if the circuit is open,
// ErrTooManyRequests if the half-open probe limit is exceeded,
// or the error returned by fn.
func (b *Breaker) Execute(fn func() error) error {
	b.mu.Lock()

	state := b.currentState()

	switch state {
	case StateOpen:
		b.mu.Unlock()
		return ErrCircuitOpen
	case StateHalfOpen:
		if b.halfOpenRequests >= b.config.MaxHalfOpenRequests {
			b.mu.Unlock()
			return ErrTooManyRequests
		}
		b.halfOpenRequests++
	}

	b.mu.Unlock()

	// Execute the function outside the lock.
	err := fn()

	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		b.recordFailure()
	} else {
		b.recordSuccess()
	}

	return err
}

// currentState evaluates and returns the current state, transitioning
// from Open to HalfOpen if the timeout has expired.
// Must be called with mu held.
func (b *Breaker) currentState() State {
	if b.state == StateOpen {
		if time.Since(b.lastFailureTime) >= b.config.Timeout {
			b.transitionTo(StateHalfOpen)
		}
	}
	return b.state
}

// recordSuccess records a successful execution.
// Must be called with mu held.
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

// recordFailure records a failed execution.
// Must be called with mu held.
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

// transitionTo changes the circuit breaker state and resets counters.
// Must be called with mu held.
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

// to is a helper to return the State value (avoids variable shadowing).
func to(s State) State { return s }

// Reset forcefully resets the circuit breaker to the Closed state.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transitionTo(StateClosed)
}
