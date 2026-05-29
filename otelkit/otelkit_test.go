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
		serviceName string
		setupCtx    func() context.Context
		wantErr     bool
		verify      func(t *testing.T, ctx context.Context, shutdown func(context.Context) error)
	}{
		{
			name:        "HappyPath_Success",
			serviceName: "test-service",
			setupCtx: func() context.Context {
				return context.Background()
			},
			wantErr: false,
			verify: func(t *testing.T, ctx context.Context, shutdown func(context.Context) error) {
				if shutdown == nil {
					t.Fatal("expected non-nil shutdown function")
				}

				// Verify global propagators were set.
				prop := otel.GetTextMapPropagator()
				if prop == nil {
					t.Fatal("expected global text map propagator to be set")
				}

				// Verify shutdown doesn't panic and behaves correctly
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if shutdownErr := shutdown(shutdownCtx); shutdownErr != nil {
					t.Errorf("shutdown() returned error: %v", shutdownErr)
				}
			},
		},
		{
			name:        "UnhappyPath_ContextCanceled",
			serviceName: "test-service-canceled",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: true,
		},
		{
			name:        "UnhappyPath_CanceledContextShutdown",
			serviceName: "test-service-shutdown-fail",
			setupCtx: func() context.Context {
				return context.Background()
			},
			wantErr: false,
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
			ctx := tt.setupCtx()

			shutdown, err := otelkit.InitSDK(ctx, tt.serviceName)

			if (err != nil) != tt.wantErr {
				t.Errorf("InitSDK() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.verify != nil {
				tt.verify(t, ctx, shutdown)
			}
		})
	}
}
