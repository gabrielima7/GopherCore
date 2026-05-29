// Package otelkit provides initialization logic for OpenTelemetry distributed tracing and metrics.
// Purpose: Bootstraps global OpenTelemetry TracerProvider and MeterProvider instances.
// Constraints: Should be called exactly once during application startup.
// Thread-safety: Setup functions are safe to be called during sequential bootstrap,
// and the returned shutdown function is thread-safe.
package otelkit

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitSDK configures the OpenTelemetry SDK with an OTLP gRPC trace exporter and a Prometheus metric exporter.
// It registers the providers globally.
// Purpose: Configures OpenTelemetry SDK for the application.
// Constraints: Must be invoked at application launch.
// Thread-safety: Global configuration should only happen once sequentially at startup.
func InitSDK(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	// Early context cancellation check
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			resource.Default().SchemaURL(),
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Set global propagators for distributed tracing.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// 1. Initialize Traces
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// 2. Initialize Metrics (Prometheus Exporter)
	metricExporter, err := prometheus.New()
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricExporter),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	shutdown := func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
	}

	return shutdown, nil
}
