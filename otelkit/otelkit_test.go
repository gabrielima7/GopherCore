package otelkit_test

import (
	"context"
	"testing"
	"time"

	"github.com/gabrielima7/GopherCore/otelkit"
	"go.opentelemetry.io/otel"
)

func TestInitSDK(t *testing.T) {
	tests := []struct {
		name        string
		setupCtx    func() (context.Context, context.CancelFunc)
		serviceName string
		wantErr     bool
		verify      func(*testing.T, context.Context, func(context.Context) error)
	}{
		{
			name: "HappyPath_SuccessfulInitialization",
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 5*time.Second)
			},
			serviceName: "test-service-happy",
			wantErr:     false,
			verify: func(t *testing.T, ctx context.Context, shutdown func(context.Context) error) {
				if shutdown == nil {
					t.Fatal("expected non-nil shutdown function")
				}

				// Verify global propagators were set.
				prop := otel.GetTextMapPropagator()
				if prop == nil {
					t.Fatal("expected global text map propagator to be set")
				}

				// Verify proper shutdown closure
				if err := shutdown(ctx); err != nil {
					t.Errorf("expected clean shutdown, got error: %v", err)
				}
			},
		},
		{
			name: "UnhappyPath_CanceledContextShutdown",
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 5*time.Second)
			},
			serviceName: "test-service-shutdown-fail",
			wantErr:     false,
			verify: func(t *testing.T, ctx context.Context, shutdown func(context.Context) error) {
				if shutdown == nil {
					t.Fatal("expected non-nil shutdown function")
				}

				cancelCtx, cancel := context.WithCancel(context.Background())
				cancel()

				err := shutdown(cancelCtx)
				if err == nil {
					t.Error("expected error from shutdown with canceled context, got nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.setupCtx()
			defer cancel()

			shutdown, err := otelkit.InitSDK(ctx, tt.serviceName)

			if (err != nil) != tt.wantErr {
				t.Fatalf("InitSDK() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.verify != nil {
				tt.verify(t, ctx, shutdown)
			}
		})
	}
}
