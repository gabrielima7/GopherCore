package circuitbreaker

import (
	"errors"
	"sync"
	"testing"
	"time"
)

var errTest = errors.New("test error")

func newTestBreaker() *Breaker {
	return New(Config{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
	})
}

func TestClosedState(t *testing.T) {
	cb := newTestBreaker()
	if cb.State() != StateClosed {
		t.Fatalf("expected Closed, got %s", cb.State())
	}

	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransitionToOpen(t *testing.T) {
	cb := newTestBreaker()

	// Trigger failure threshold.
	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return errTest })
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected Open, got %s", cb.State())
	}

	err := cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got: %v", err)
	}
}

func TestTransitionToHalfOpen(t *testing.T) {
	cb := newTestBreaker()

	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return errTest })
	}

	// Wait for timeout.
	time.Sleep(60 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HalfOpen, got %s", cb.State())
	}
}

func TestHalfOpenToClosedOnSuccess(t *testing.T) {
	cb := New(Config{
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 3,
	})

	// Trip the breaker.
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error { return errTest })
	}
	time.Sleep(60 * time.Millisecond)

	// Two successes should close it.
	for i := 0; i < 2; i++ {
		err := cb.Execute(func() error { return nil })
		if err != nil {
			t.Fatalf("unexpected error on attempt %d: %v", i, err)
		}
	}

	if cb.State() != StateClosed {
		t.Fatalf("expected Closed, got %s", cb.State())
	}
}

func TestHalfOpenToOpenOnFailure(t *testing.T) {
	cb := newTestBreaker()

	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return errTest })
	}
	time.Sleep(60 * time.Millisecond)

	// A failure in HalfOpen should re-open.
	_ = cb.Execute(func() error { return errTest })

	if cb.State() != StateOpen {
		t.Fatalf("expected Open, got %s", cb.State())
	}
}

func TestTooManyRequestsInHalfOpen(t *testing.T) {
	cb := newTestBreaker() // MaxHalfOpenRequests = 1

	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return errTest })
	}
	time.Sleep(60 * time.Millisecond)

	// To test "TooManyRequests", we need concurrent requests before the first one finishes,
	// because now Execute properly decrements halfOpenRequests upon return.
	// We'll block the first request so the second one hits the limit.
	ch := make(chan struct{})

	// Also use a waitgroup so the test doesn't exit before the goroutine finishes
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = cb.Execute(func() error {
			<-ch
			return nil
		})
	}()

	// Wait briefly to ensure the goroutine enters the Execute block and increments the counter
	time.Sleep(10 * time.Millisecond)

	// Second request should be rejected immediately
	err := cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrTooManyRequests) {
		t.Fatalf("expected ErrTooManyRequests, got: %v", err)
	}

	close(ch)
	wg.Wait()
}

func TestReset_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(*testing.T, *Breaker)
		expectedState State
	}{
		{
			name: "Reset from Open State",
			setup: func(t *testing.T, cb *Breaker) {
				for i := 0; i < 3; i++ {
					_ = cb.Execute(func() error { return errTest })
				}
				if cb.State() != StateOpen {
					t.Fatalf("expected Open state before reset, got %v", cb.State())
				}
			},
			expectedState: StateClosed,
		},
		{
			name: "Reset from HalfOpen State",
			setup: func(t *testing.T, cb *Breaker) {
				for i := 0; i < 3; i++ {
					_ = cb.Execute(func() error { return errTest })
				}
				cb.mu.Lock()
				cb.lastFailureTime = time.Now().Add(-100 * time.Millisecond) // force timeout expiration
				cb.mu.Unlock()
				if cb.State() != StateHalfOpen {
					t.Fatalf("expected HalfOpen state before reset, got %v", cb.State())
				}
			},
			expectedState: StateClosed,
		},
		{
			name: "Reset from Closed State",
			setup: func(t *testing.T, cb *Breaker) {
				if cb.State() != StateClosed {
					t.Fatalf("expected Closed state before reset, got %v", cb.State())
				}
			},
			expectedState: StateClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := newTestBreaker()
			tt.setup(t, cb)
			cb.Reset()
			if cb.State() != tt.expectedState {
				t.Errorf("expected %v after Reset, got %v", tt.expectedState, cb.State())
			}
		})
	}
}

func TestOnStateChange(t *testing.T) {
	var transitions []struct{ from, to State }
	cb := New(Config{
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
		OnStateChange: func(from, to State) {
			transitions = append(transitions, struct{ from, to State }{from, to})
		},
	})

	// Closed → Open
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error { return errTest })
	}

	if len(transitions) != 1 || transitions[0].from != StateClosed || transitions[0].to != StateOpen {
		t.Fatalf("unexpected transitions: %+v", transitions)
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(99), "unknown"},
	}
	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, tt.state.String())
		}
	}
}

func TestBreaker_Execute_TableDriven(t *testing.T) {
	errGeneric := errors.New("generic error")

	tests := []struct {
		name          string
		initialState  State
		halfOpenReqs  int
		lastFailTime  time.Time
		timeout       time.Duration
		executeFnErr  error
		expectedErr   error
		expectedState State
		validate      func(*testing.T, *Breaker)
	}{
		{
			name:          "StateOpen fast fails with ErrCircuitOpen",
			initialState:  StateOpen,
			lastFailTime:  time.Now(),
			timeout:       50 * time.Millisecond,
			executeFnErr:  nil, // Should not be called
			expectedErr:   ErrCircuitOpen,
			expectedState: StateOpen,
		},
		{
			name:          "StateOpen transitions to StateHalfOpen when timeout expires",
			initialState:  StateOpen,
			lastFailTime:  time.Now().Add(-100 * time.Millisecond),
			timeout:       50 * time.Millisecond,
			executeFnErr:  nil,
			expectedErr:   nil,
			expectedState: StateHalfOpen, // After 1 success it doesn't immediately close (SuccessThreshold is 2)
			validate: func(t *testing.T, b *Breaker) {
				if b.successCount != 1 {
					t.Errorf("expected 1 success count, got %d", b.successCount)
				}
			},
		},
		{
			name:          "StateHalfOpen rejects request if MaxHalfOpenRequests exceeded",
			initialState:  StateHalfOpen,
			halfOpenReqs:  2,   // Config MaxHalfOpenRequests is 2
			executeFnErr:  nil, // Should not be called
			expectedErr:   ErrTooManyRequests,
			expectedState: StateHalfOpen,
		},
		{
			name:          "StateHalfOpen transitions to StateOpen on failure",
			initialState:  StateHalfOpen,
			halfOpenReqs:  0,
			executeFnErr:  errGeneric,
			expectedErr:   errGeneric,
			expectedState: StateOpen,
		},
		{
			name:          "StateClosed transitions to StateOpen on reaching failure threshold",
			initialState:  StateClosed,
			executeFnErr:  errGeneric,
			expectedErr:   errGeneric,
			expectedState: StateOpen,
			validate: func(t *testing.T, b *Breaker) {
				// We inject 2 initial failures, then the execution causes the 3rd, hitting the threshold of 3.
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeout := tt.timeout
			if timeout == 0 {
				timeout = 50 * time.Millisecond
			}

			cb := New(Config{
				FailureThreshold:    3,
				SuccessThreshold:    2,
				Timeout:             timeout,
				MaxHalfOpenRequests: 2,
			})

			// Force internal state
			cb.mu.Lock()
			cb.state = tt.initialState
			cb.halfOpenRequests = tt.halfOpenReqs
			cb.lastFailureTime = tt.lastFailTime
			if tt.name == "StateClosed transitions to StateOpen on reaching failure threshold" {
				cb.failureCount = 2 // 1 failure away from threshold
			}
			cb.mu.Unlock()

			err := cb.Execute(func() error {
				return tt.executeFnErr
			})

			if tt.expectedErr == nil {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if !errors.Is(err, tt.expectedErr) {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
			}

			if cb.State() != tt.expectedState {
				t.Errorf("expected state %s, got %s", tt.expectedState, cb.State())
			}

			if tt.validate != nil {
				cb.mu.Lock()
				tt.validate(t, cb)
				cb.mu.Unlock()
			}
		})
	}
}

func TestConcurrentExecute(t *testing.T) {
	cb := New(Config{
		FailureThreshold:    100,
		SuccessThreshold:    1,
		Timeout:             time.Second,
		MaxHalfOpenRequests: 10,
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cb.Execute(func() error { return nil })
		}()
	}
	wg.Wait()

	if cb.State() != StateClosed {
		t.Fatalf("expected Closed after concurrent successes, got %s", cb.State())
	}
}

func TestConfig_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		input    Config
		expected Config
	}{
		{
			name:  "DefaultConfig",
			input: DefaultConfig(),
			expected: Config{
				FailureThreshold:    5,
				SuccessThreshold:    2,
				Timeout:             30 * time.Second,
				MaxHalfOpenRequests: 1,
			},
		},
		{
			name:  "Zero values",
			input: Config{},
			expected: Config{
				FailureThreshold:    5,
				SuccessThreshold:    2,
				Timeout:             30 * time.Second,
				MaxHalfOpenRequests: 1,
			},
		},
		{
			name: "Negative values",
			input: Config{
				FailureThreshold:    -1,
				SuccessThreshold:    -1,
				Timeout:             -1,
				MaxHalfOpenRequests: -1,
			},
			expected: Config{
				FailureThreshold:    5,
				SuccessThreshold:    2,
				Timeout:             30 * time.Second,
				MaxHalfOpenRequests: 1,
			},
		},
		{
			name: "Valid values",
			input: Config{
				FailureThreshold:    10,
				SuccessThreshold:    5,
				Timeout:             10 * time.Second,
				MaxHalfOpenRequests: 3,
			},
			expected: Config{
				FailureThreshold:    10,
				SuccessThreshold:    5,
				Timeout:             10 * time.Second,
				MaxHalfOpenRequests: 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := New(tt.input)
			if cb.config.FailureThreshold != tt.expected.FailureThreshold {
				t.Errorf("expected FailureThreshold %d, got %d", tt.expected.FailureThreshold, cb.config.FailureThreshold)
			}
			if cb.config.SuccessThreshold != tt.expected.SuccessThreshold {
				t.Errorf("expected SuccessThreshold %d, got %d", tt.expected.SuccessThreshold, cb.config.SuccessThreshold)
			}
			if cb.config.Timeout != tt.expected.Timeout {
				t.Errorf("expected Timeout %v, got %v", tt.expected.Timeout, cb.config.Timeout)
			}
			if cb.config.MaxHalfOpenRequests != tt.expected.MaxHalfOpenRequests {
				t.Errorf("expected MaxHalfOpenRequests %d, got %d", tt.expected.MaxHalfOpenRequests, cb.config.MaxHalfOpenRequests)
			}
		})
	}
}

func TestTransitionToSameStateIsNoOp(t *testing.T) {
	called := false
	cb := New(Config{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
		OnStateChange: func(from, to State) {
			called = true
		},
	})
	// Reset to Closed (already Closed) — should be a no-op.
	cb.Reset()

	// Force explicit transition to the exact same state (closed) via internals
	cb.mu.Lock()
	cb.transitionTo(StateClosed)
	cb.mu.Unlock()

	if called {
		t.Fatal("OnStateChange should NOT be called when transitioning to same state")
	}
}

func TestExecuteSuccessInClosedResetsFailures(t *testing.T) {
	cb := newTestBreaker()

	// Add some failures (below threshold).
	_ = cb.Execute(func() error { return errTest })
	_ = cb.Execute(func() error { return errTest })

	// Success resets failure count.
	_ = cb.Execute(func() error { return nil })

	// Now 3 more failures from zero should trip it.
	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return errTest })
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected Open after threshold failures, got %s", cb.State())
	}
}

func TestNoOnStateChangeCallback(t *testing.T) {
	// Config without OnStateChange — should not panic.
	cb := New(Config{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
	})
	_ = cb.Execute(func() error { return errTest })
	if cb.State() != StateOpen {
		t.Fatalf("expected Open, got %s", cb.State())
	}
}

func TestFullCycleClosedOpenHalfOpenClosed(t *testing.T) {
	var stateLog []string
	cb := New(Config{
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
		OnStateChange: func(from, to State) {
			stateLog = append(stateLog, from.String()+"→"+to.String())
		},
	})

	// Closed → Open
	_ = cb.Execute(func() error { return errTest })
	_ = cb.Execute(func() error { return errTest })

	// Wait for Open → HalfOpen
	time.Sleep(60 * time.Millisecond)
	_ = cb.State() // trigger transition

	// HalfOpen → Closed (success)
	_ = cb.Execute(func() error { return nil })

	expected := []string{"closed→open", "open→half-open", "half-open→closed"}
	if len(stateLog) != len(expected) {
		t.Fatalf("expected %d transitions, got %d: %v", len(expected), len(stateLog), stateLog)
	}
	for i, exp := range expected {
		if stateLog[i] != exp {
			t.Fatalf("transition %d: expected %q, got %q", i, exp, stateLog[i])
		}
	}
}

func FuzzBreakerThresholds(f *testing.F) {
	f.Add(3, 2, 5)
	f.Fuzz(func(t *testing.T, failThresh, successThresh, ops int) {
		if failThresh <= 0 || failThresh > 100 || successThresh <= 0 || successThresh > 100 || ops < 0 || ops > 200 {
			return
		}
		cb := New(Config{
			FailureThreshold:    failThresh,
			SuccessThreshold:    successThresh,
			Timeout:             time.Millisecond,
			MaxHalfOpenRequests: successThresh,
		})
		for i := 0; i < ops; i++ {
			_ = cb.Execute(func() error {
				if i%2 == 0 {
					return errTest
				}
				return nil
			})
		}
		// Should not panic — that's the main assertion.
		_ = cb.State()
	})
}

func TestExecutePanic(t *testing.T) {
	cb := newTestBreaker()

	// Ensure a panic inside Execute records a failure and propagates the panic.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, got none")
		}

		// The panic should be counted as a failure, so failure count should be 1.
		cb.mu.Lock()
		defer cb.mu.Unlock()
		if cb.failureCount != 1 {
			t.Errorf("expected failureCount 1 after panic, got %d", cb.failureCount)
		}
	}()

	_ = cb.Execute(func() error {
		panic("catastrophic failure")
	})
}
