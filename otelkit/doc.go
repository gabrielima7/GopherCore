// Package otelkit provides initialization logic for OpenTelemetry distributed tracing and metrics.
// Purpose: Bootstraps global OpenTelemetry TracerProvider and MeterProvider instances.
// Constraints: Should be called exactly once during application startup.
// Thread-safety: Setup functions are safe to be called during sequential bootstrap,
// and the returned shutdown function is thread-safe.
package otelkit
