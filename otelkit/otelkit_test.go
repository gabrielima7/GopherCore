package otelkit_test

import (
	"context"
	"testing"
	"time"

	"github.com/gabrielima7/GopherCore/otelkit"
)

func TestInitSDK(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		setupCtx    func() context.Context
		wantErr     bool
	}{
		{
			name:        "HappyPath_Success",
			serviceName: "test-service",
			setupCtx: func() context.Context {
				return context.Background()
			},
			wantErr: false,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()

			shutdown, err := otelkit.InitSDK(ctx, tt.serviceName)

			if (err != nil) != tt.wantErr {
				t.Errorf("InitSDK() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if shutdown == nil {
					t.Error("InitSDK() returned nil shutdown function without an error")
				} else {
					// Verify shutdown doesn't panic and behaves correctly
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					if shutdownErr := shutdown(shutdownCtx); shutdownErr != nil {
						t.Errorf("shutdown() returned error: %v", shutdownErr)
					}
				}
			}
		})
	}
}
