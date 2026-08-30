// Package otelkit provides utilities.
// Purpose: otelkit provides OpenTelemetry tracing and metrics bootstrapping.
// Constraints: Internal package.
// Thread-safety: Varies by component.
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

var (
	// resourceMerge is an internal override hook.
	// Purpose: Allows mocking OpenTelemetry resource merging in unit tests.
	// Constraints: Should only be reassigned during test setup.
	// Thread-safety: Unsafe to modify concurrently.
	resourceMerge = resource.Merge

	// newTraceExporter is an internal override hook.
	// Purpose: Allows mocking the OTLP trace exporter in unit tests.
	// Constraints: Should only be reassigned during test setup.
	// Thread-safety: Unsafe to modify concurrently.
	newTraceExporter = otlptracegrpc.New

	// newMetricExporter is an internal override hook.
	// Purpose: Allows mocking the Prometheus metric exporter in unit tests.
	// Constraints: Should only be reassigned during test setup.
	// Thread-safety: Unsafe to modify concurrently.
	newMetricExporter = prometheus.New
)

// InitSDK configures the OpenTelemetry SDK with an OTLP gRPC trace exporter and a Prometheus metric exporter.
// It registers the providers globally.
// Purpose: Configures OpenTelemetry SDK for the application.
// Constraints: Must be invoked at application launch.
// Thread-safety: Global configuration should only happen once sequentially at startup.
// Internal Logic Deep-Dive: We rigorously initialize the trace exporter and provider, registering them as globals. This guarantees all downstream packages immediately emit telemetry data without needing direct reference to the SDK.
func InitSDK(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	// Early context cancellation check
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	res, err := resourceMerge(
		resource.Default(),
		resource.NewWithAttributes(
			resource.Default().SchemaURL(),
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Internal Logic Deep-Dive: We must explicitly set a global TextMapPropagator so that the OpenTelemetry instrumentation natively understands how to inject and extract `traceparent` and `baggage` headers across network boundaries. Without this, distributed tracing breaks immediately when a request leaves this microservice to call another.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// 1. Initialize Traces
	traceExporter, err := newTraceExporter(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// 2. Initialize Metrics (Prometheus Exporter)
	metricExporter, err := newMetricExporter()
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
