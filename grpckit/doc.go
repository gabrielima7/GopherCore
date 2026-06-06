// Package grpckit provides a productive and secure abstraction over the
// google.golang.org/grpc library, containing production-ready utilities for
// both gRPC Servers and Clients.
//
// It enforces mandatory panic recovery and structured request logging on every
// server by default, while exposing a clean Functional Options API for TLS,
// address binding, and custom interceptor injection. Clients are configured
// with sensible dial timeouts and optional TLS.
// Purpose: Provide an ergonomic, secure, and observable wrapper around standard gRPC.
// Constraints: Assumes usage with github.com/grpc/grpc-go.
// Thread-safety: Safe for concurrent use across multiple goroutines.
//
// Internal Logic Deep-Dive:
// GopherCore integrates OpenTelemetry distributed tracing natively using the modern
// `grpc.StatsHandler` API (via `otelgrpc.NewServerHandler()` and `otelgrpc.NewClientHandler()`)
// rather than legacy unary/stream interceptors. This ensures complete observability coverage
// extending into connection lifecycle events, reducing middleware chain complexity while
// remaining fully spec-compliant.
package grpckit
