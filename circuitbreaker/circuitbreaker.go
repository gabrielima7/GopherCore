// Package circuitbreaker provides utilities.
// Purpose: circuitbreaker provides an implementation of the circuit breaker pattern to prevent cascading failures.
// Constraints: Internal package.
// Thread-safety: Varies by component.
package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen serves as an immediate terminal sentinel raised whenever execution is blocked by a tripped breaker, commanding upstream callers to rapidly failover.
// Purpose: Indicates that a circuit is fully tripped and cannot process requests.
// Constraints: Treat as a sentinel error for matching with errors.Is.
// Thread-safety: Pure error sentinel, safe for concurrent use.
var ErrCircuitOpen = errors.New("circuitbreaker: circuit is open")

// ErrTooManyRequests actively repels secondary requests hitting a recovering half-open circuit once the strict probing ceiling is saturated, shielding the fragile downstream service.
// Purpose: Prevents overwhelming a recovering service with too many probes.
// Constraints: Treat as a sentinel error for matching with errors.Is.
// Thread-safety: Pure error sentinel, safe for concurrent use.
var ErrTooManyRequests = errors.New("circuitbreaker: too many requests in half-open state")

// State calculates the logical operational phase of the breaker machine, directly controlling whether network traffic flows cleanly or is aggressively blackholed.
// Purpose: Used to determine if requests should be allowed, rejected, or probed.
// Constraints: Should only be one of the defined constants.
// Thread-safety: Pure enum.
type State int

const (
	// StateClosed represents the normal operational state. All requests are allowed
	// through. The breaker counts consecutive failures to determine if it
	// should trip to StateOpen.
	// Purpose: Denotes the baseline healthy state.
	// Constraints: Must be returned exclusively when the breaker is untripped.
	// Thread-safety: Constant value.
	StateClosed	State	= iota

	// StateOpen represents the tripped state. All requests are immediately rejected
	// with ErrCircuitOpen until the configured timeout duration expires.
	// Purpose: Denotes the failing, protective state.
	// Constraints: Must enforce fast-failure rejections.
	// Thread-safety: Constant value.
	StateOpen

	// StateHalfOpen represents the recovery state. A limited number of probe requests
	// are allowed through to test if the underlying service has recovered.
	// Purpose: Denotes the tentative recovery state.
	// Constraints: Must restrict the number of probes to avoid re-overloading.
	// Thread-safety: Constant value.
	StateHalfOpen
)

// String maps the internal integer phase enum into an intuitive, human-parseable text label, heavily utilized during metric exports and structured observability logging.
// Purpose: Simplifies console output and logging of the circuit status.
// Constraints: Always returns a valid string, defaulting to "unknown".
// Thread-safety: Pure method on value receiver.
func (s State) String() string {
	// Directly convert enum values into human-readable representations.
	// This makes it vastly simpler to aggregate and query circuit breaker
	// statuses securely in metrics dashboards (like Prometheus/Grafana) and logging pipelines.
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

// Config consolidates the sensitive tuning gauges controlling precisely how resiliently or aggressively the circuit breaker reacts to a sustained barrage of external execution failures.
// Purpose: Defines operational limits like timeout and failure thresholds.
// Constraints: All numeric fields must be strictly positive or will be set to defaults.
// Thread-safety: Treat as read-only once passed to the Breaker constructor.
type Config struct {
	// FailureThreshold is the number of consecutive failures before
	// the circuit breaker transitions from Closed to Open.
	// Purpose: Determines how many consecutive failures trip the breaker.
	// Constraints: Must be greater than 0.
	// Thread-safety: Read-only during execution.
	FailureThreshold	int

	// SuccessThreshold is the number of consecutive successes in
	// HalfOpen state required to transition back to Closed.
	// Purpose: Determines how many consecutive successes reset the breaker.
	// Constraints: Must be greater than 0.
	// Thread-safety: Read-only during execution.
	SuccessThreshold	int

	// Timeout is the duration the circuit stays in the Open state
	// before transitioning to HalfOpen.
	// Purpose: Determines the cooldown period before probing the service again.
	// Constraints: Must be greater than 0.
	// Thread-safety: Read-only during execution.
	Timeout	time.Duration

	// MaxHalfOpenRequests is the maximum number of requests allowed
	// in the HalfOpen state. Defaults to 1.
	// Purpose: Limits concurrent probes to the recovering service.
	// Constraints: Must be greater than 0.
	// Thread-safety: Read-only during execution.
	MaxHalfOpenRequests	int

	// OnStateChange is called when the circuit breaker transitions state.
	// Purpose: Allows observing internal circuit breaker state changes.
	// Constraints: Can be nil. If provided, it blocks state transitions.
	// Thread-safety: Called synchronously under the breaker's mutex lock.
	OnStateChange	func(from, to State)
}

// DefaultConfig establishes a highly battle-tested, conservative tolerance foundation designed to safely protect the majority of standard microservice architectures against cascading network death.
// Purpose: Provides a safe, battle-tested baseline configuration.
// Constraints: Generates defaults that can be optionally overridden.
// Thread-safety: Returns a new instance.
func DefaultConfig() Config {
	return Config{
		FailureThreshold:	5,
		SuccessThreshold:	2,
		Timeout:		30 * time.Second,
		MaxHalfOpenRequests:	1,
	}
}

// Breaker encapsulates a finite state machine that automatically opens when failure thresholds are exceeded, rejecting requests to allow the downstream service to recover.
// Purpose: Prevents cascading failures by managing circuit state and statistics.
// Constraints: Must be created using New() and never copied by value after initialization.
// Thread-safety: Mutex-guarded and safe for concurrent use.
type Breaker struct {
	mu	sync.Mutex
	config	Config

	state			State
	failureCount		int
	successCount		int
	halfOpenRequests	int
	lastFailureTime		time.Time
}

// New creates a new Breaker instance with the provided Config.
// Purpose: Instantiates and preconfigures a new Circuit Breaker.
// Constraints: Applies default values for any configuration fields that are zero or invalid (<= 0). The breaker starts in StateClosed.
// Thread-safety: Safe to initialize.
func New(cfg Config) *Breaker {
	// Sanitize configuration arguments silently rather than panicking or failing.
	// This defensive posture ensures the circuit breaker guarantees system resilience
	// even if injected configuration sources feed it partially malformed definitions.
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

// State determines the internal temporal tracking mechanics beneath an active mutex lock, exposing the exact calculated operational status of the gateway in real time.
// Purpose: Allows synchronous querying of the active circuit phase.
// Constraints: It handles potential state transitions (e.g., from Open to HalfOpen) if the timeout
// has expired before returning the state.
// Thread-safety: Safe for concurrent use, heavily guarded by the internal mutex.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentState()
}

// Execute protects the execution of the user-provided function fn.
// Purpose: Rejects requests when the circuit is Open or too busy in HalfOpen, otherwise runs fn and tracks outcomes.
// Constraints: Returns ErrCircuitOpen when Open, ErrTooManyRequests when HalfOpen limit is reached.
// Thread-safety: Safe for concurrent use; releases the internal lock during execution of fn.
// Internal Logic Deep-Dive: The state machine transitions atomically. If the circuit is open, we fast-fail returning ErrCircuitOpen to prevent cascading failure pressure on the downstream service.
func (b *Breaker) Execute(fn func() error) error {
	b.mu.Lock()

	// Evaluate the state lazily when traffic arrives. This prevents us from
	// needing background worker goroutines to constantly evaluate timeout expirations.
	state := b.currentState()

	var isHalfOpenProbe bool
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
		isHalfOpenProbe = true
	}

	// Deliberately drop the mutex before calling the external system. Holding it here
	// would serialize all execution throughput, turning the breaker into an extreme bottleneck.
	// Internal Logic Deep-Dive: We drop the `b.mu.Unlock()` lock here explicitly before executing `fn()`. If we held the lock while waiting for the downstream network call to finish, every single parallel HTTP request passing through this circuit breaker would execute sequentially, catastrophically destroying system concurrency and throughput.
	b.mu.Unlock()

	var err error
	panicked := true

	defer func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		if isHalfOpenProbe {
			b.halfOpenRequests--
		}

		if panicked {
			// A panic is considered a catastrophic failure,
			// so we record it before the panic continues bubbling up.
			b.recordFailure()
		} else if err != nil {
			b.recordFailure()
		} else {
			b.recordSuccess()
		}
	}()

	err = fn()
	panicked = false

	return err
}

// currentState evaluates and returns the current state.
// Purpose: Computes the actual logical state, taking timeout expirations into account.
//
// Constraints: If the state is Open, it checks if the timeout duration has elapsed
// since the last failure. If so, it automatically transitions the state
// to HalfOpen to allow probe requests.
// Thread-safety: This function REQUIRES the Breaker's mutex to be strictly held by the caller to avoid panics.
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
// Purpose: Manages state transitions after successful probes or executions.
//
// Constraints: In the Closed state, it resets the consecutive failure count.
// In the HalfOpen state, it increments the success count and transitions
// back to Closed if the success threshold is met.
// Thread-safety: This function REQUIRES the Breaker's mutex to be strictly held by the caller.
func (b *Breaker) recordSuccess() {
	// A successful execution resets accumulated failure metrics differently
	// depending on the active phase. In a tentative HalfOpen state, we require
	// consistent consecutive successes before officially declaring the network healthy.
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
// Purpose: Manages state transitions after a failed execution.
//
// Constraints: It records the time of the failure. In the Closed state, it increments the
// failure count and transitions to Open if the threshold is reached.
// In the HalfOpen state, any failure immediately trips the circuit back to Open.
// Thread-safety: This function REQUIRES the Breaker's mutex to be strictly held by the caller.
func (b *Breaker) recordFailure() {
	b.lastFailureTime = time.Now()

	// Handle failures aggressively based on the current state. If in HalfOpen,
	// even a single failure confirms the underlying service is still broken,
	// immediately tripping the circuit back to Open to shield the network.
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
// Purpose: Applies a complete internal state transition.
//
// Constraints: If a state change callback is configured, it is invoked synchronously.
// Thread-safety: This function REQUIRES the Breaker's mutex to be strictly held by the caller.
func (b *Breaker) transitionTo(newState State) {
	if b.state == newState {
		return
	}

	// Save the old state, assign the new state, and then reset all volatile
	// metrics tracking historical behavior so that the new phase starts cleanly.
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
// Purpose: It is used to bypass variable shadowing issues in closure contexts.
// Constraints: Must only be used internally.
// Thread-safety: Pure function.
func to(s State) State	{ return s }

// Reset restores the circuit breaker to its closed state, clearing all statistics.
// Purpose: Manually clears any failure conditions.
// Constraints: Disregards threshold counts when invoked.
// Thread-safety: Mutex-locked and safe for concurrent use.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transitionTo(StateClosed)
}
