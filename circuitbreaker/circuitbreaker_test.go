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

	// First request in half-open is allowed.
	_ = cb.Execute(func() error { return nil })

	// Second request should be rejected.
	err := cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrTooManyRequests) {
		t.Fatalf("expected ErrTooManyRequests, got: %v", err)
	}
}

func TestReset(t *testing.T) {
	cb := newTestBreaker()

	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return errTest })
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected Open, got %s", cb.State())
	}

	cb.Reset()
	if cb.State() != StateClosed {
		t.Fatalf("expected Closed after Reset, got %s", cb.State())
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

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.FailureThreshold != 5 {
		t.Fatalf("expected 5, got %d", cfg.FailureThreshold)
	}
	if cfg.SuccessThreshold != 2 {
		t.Fatalf("expected 2, got %d", cfg.SuccessThreshold)
	}
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("expected 30s, got %v", cfg.Timeout)
	}
	if cfg.MaxHalfOpenRequests != 1 {
		t.Fatalf("expected 1, got %d", cfg.MaxHalfOpenRequests)
	}
}

func TestNewWithZeroConfig(t *testing.T) {
	// All zero values should get defaults.
	cb := New(Config{})
	if cb.config.FailureThreshold != 5 {
		t.Fatalf("expected default 5, got %d", cb.config.FailureThreshold)
	}
	if cb.config.SuccessThreshold != 2 {
		t.Fatalf("expected default 2, got %d", cb.config.SuccessThreshold)
	}
	if cb.config.Timeout != 30*time.Second {
		t.Fatalf("expected default 30s, got %v", cb.config.Timeout)
	}
	if cb.config.MaxHalfOpenRequests != 1 {
		t.Fatalf("expected default 1, got %d", cb.config.MaxHalfOpenRequests)
	}
}

func TestNewWithNegativeConfig(t *testing.T) {
	cb := New(Config{
		FailureThreshold:    -1,
		SuccessThreshold:    -1,
		Timeout:             -1,
		MaxHalfOpenRequests: -1,
	})
	if cb.config.FailureThreshold != 5 {
		t.Fatalf("expected default 5, got %d", cb.config.FailureThreshold)
	}
	if cb.config.SuccessThreshold != 2 {
		t.Fatalf("expected default 2, got %d", cb.config.SuccessThreshold)
	}
	if cb.config.Timeout != 30*time.Second {
		t.Fatalf("expected default 30s, got %v", cb.config.Timeout)
	}
	if cb.config.MaxHalfOpenRequests != 1 {
		t.Fatalf("expected default 1, got %d", cb.config.MaxHalfOpenRequests)
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
