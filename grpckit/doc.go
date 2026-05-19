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
package grpckit
