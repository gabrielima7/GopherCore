package otelkit

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/resource"
)

func TestInitSDK(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		setupCtx    func() context.Context
		setupMock   func()
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
		{
			name:        "UnhappyPath_ResourceMergeError",
			serviceName: "test-service-resource-error",
			setupCtx: func() context.Context {
				return context.Background()
			},
			setupMock: func() {
				SetResourceMerge(func(r1, r2 *resource.Resource) (*resource.Resource, error) {
					return nil, errors.New("simulated resource merge error")
				})
			},
			wantErr: true,
		},
		{
			name:        "UnhappyPath_TraceExporterError",
			serviceName: "test-service-trace-error",
			setupCtx: func() context.Context {
				return context.Background()
			},
			setupMock: func() {
				SetNewTraceExporter(func(ctx context.Context, opts ...otlptracegrpc.Option) (*otlptrace.Exporter, error) {
					return nil, errors.New("simulated trace exporter error")
				})
			},
			wantErr: true,
		},
		{
			name:        "UnhappyPath_MetricExporterError",
			serviceName: "test-service-metric-error",
			setupCtx: func() context.Context {
				return context.Background()
			},
			setupMock: func() {
				SetNewMetricExporter(func(opts ...prometheus.Option) (*prometheus.Exporter, error) {
					return nil, errors.New("simulated metric exporter error")
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupMock != nil {
				tt.setupMock()
				defer func() {
					RestoreResourceMerge()
					RestoreNewTraceExporter()
					RestoreNewMetricExporter()
				}()
			}

			ctx := tt.setupCtx()

			shutdown, err := InitSDK(ctx, tt.serviceName)

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
