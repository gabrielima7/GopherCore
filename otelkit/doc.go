// Package otelkit provides initialization logic for OpenTelemetry distributed tracing and metrics.
// Purpose: Bootstraps global OpenTelemetry TracerProvider and MeterProvider instances.
// Constraints: Should be called exactly once during application startup.
// Thread-safety: Setup functions are safe to be called during sequential bootstrap,
// and the returned shutdown function is thread-safe.
// Internal Logic Deep-Dive: OpenTelemetry SDK initialization strictly blocks during startup to guarantee all trace propagators and exporters are fully connected before the server binds to its port, preventing silent drop of early traffic traces.
package otelkit
