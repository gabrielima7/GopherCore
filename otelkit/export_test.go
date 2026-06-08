package otelkit

import (
	"context"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Export variables for testing
func SetResourceMerge(f func(r1, r2 *resource.Resource) (*resource.Resource, error)) {
	resourceMerge = f
}

func RestoreResourceMerge() {
	resourceMerge = resource.Merge
}

func SetNewTraceExporter(f func(ctx context.Context, opts ...otlptracegrpc.Option) (*otlptrace.Exporter, error)) {
	newTraceExporter = f
}

func RestoreNewTraceExporter() {
	newTraceExporter = otlptracegrpc.New
}

func SetNewMetricExporter(f func(opts ...prometheus.Option) (*prometheus.Exporter, error)) {
	newMetricExporter = f
}

func RestoreNewMetricExporter() {
	newMetricExporter = prometheus.New
}
