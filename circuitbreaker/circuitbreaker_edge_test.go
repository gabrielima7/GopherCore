package circuitbreaker_test

import (
	"context"
	"testing"
	"go.uber.org/goleak"
	"github.com/gabrielima7/GopherCore/circuitbreaker"
)

func TestStateString_Unknown(t *testing.T) {
	defer goleak.VerifyNone(t)
	tests := []struct {
		name     string
		state    circuitbreaker.State
		expected string
	}{
		{
			name:     "unknown state",
			state:    circuitbreaker.State(999),
			expected: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, got)
			}
		})
	}
}

func TestExecuteContext_NilCtx(t *testing.T) {
	defer goleak.VerifyNone(t)
	cb := circuitbreaker.New(circuitbreaker.Config{})

	tests := []struct {
		name string
		ctx  context.Context
	}{
		{
			name: "untyped nil context",
			ctx:  nil,
		},
		{
			name: "typed nil context",
			ctx:  (context.Context)(nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cb.ExecuteContext(tt.ctx, func() error {
				return nil
			})
			if err != nil {
				t.Fatalf("expected nil err, got %v", err)
			}
		})
	}
}
