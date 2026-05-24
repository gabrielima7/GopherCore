package grpckit

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ─────────────────────────────────────────────────────────────────────────────
// Recovery Interceptors
// ─────────────────────────────────────────────────────────────────────────────

// RecoveryUnaryInterceptor returns a gRPC unary server interceptor that
// catches any panic emitted by a downstream handler, emits a structured error
// log entry via the provided logger, and translates the panic value into a
// gRPC error carrying codes.Internal — guaranteeing the server process never
// crashes due to an unhandled panic in handler code.
//
// The log record includes:
//   - "grpc.method"  — full gRPC method path from the incoming context.
//   - "panic.value"  — the recovered panic value as a formatted string.
//   - "stack"        — the goroutine stack trace captured at the panic site.
//
// Purpose: Acts as the outermost safety net in the interceptor chain,
// preventing any single handler panic from bringing down the entire server.
// Constraints: Must be the first interceptor in the chain so that panics
// produced by downstream interceptors are also captured.
// Thread-safety: The returned interceptor is safe for concurrent invocation
// across thousands of simultaneous RPCs. logger must be safe for concurrent
// use (all *slog.Logger implementations satisfy this).
func RecoveryUnaryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		// Immediately sever the connection if the client has disconnected or timed out.
		if err := ctx.Err(); err != nil {
			return nil, status.FromContextError(err).Err()
		}

		// We defer a recovery function to catch any panics from downstream handlers.
		// This guarantees that the gRPC server process stays alive even if an individual
		// request encounters a critical bug, converting the panic into an Internal server error.
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				logger.ErrorContext(ctx, "grpc: unary handler panic recovered",
					slog.String("grpc.method", info.FullMethod),
					slog.String("panic.value", fmt.Sprintf("%v", r)),
					slog.String("stack", string(stack)),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// RecoveryStreamInterceptor returns a gRPC stream server interceptor that
// catches any panic emitted by a downstream stream handler, emits a structured
// error log entry via the provided logger, and translates the panic value into a
// gRPC error carrying codes.Internal.
//
// The log record mirrors the structure of RecoveryUnaryInterceptor with
// "grpc.method", "panic.value", and "stack" attributes.
//
// Purpose: Provides the same panic-safety guarantee as RecoveryUnaryInterceptor
// for bidirectional, server-side, and client-side streaming RPCs.
// Constraints: Must be the first stream interceptor in the chain.
// Thread-safety: The returned interceptor is safe for concurrent invocation
// across simultaneous streaming RPCs.
func RecoveryStreamInterceptor(logger *slog.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		// Immediately sever the connection if the client has disconnected or timed out.
		if err := ss.Context().Err(); err != nil {
			return status.FromContextError(err).Err()
		}

		// Similar to the unary interceptor, this deferred function catches and isolates
		// panics within streaming RPCs. It ensures streaming handlers can fail safely
		// without compromising the stability of other concurrent streams on the server.
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				logger.ErrorContext(ss.Context(), "grpc: stream handler panic recovered",
					slog.String("grpc.method", info.FullMethod),
					slog.String("panic.value", fmt.Sprintf("%v", r)),
					slog.String("stack", string(stack)),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Logging Interceptors
// ─────────────────────────────────────────────────────────────────────────────

// LoggingUnaryInterceptor returns a gRPC unary server interceptor that emits
// a single structured log entry per RPC, capturing the full method path,
// wall-clock duration, and final gRPC status code.
//
// Log records are emitted at INFO level for successful calls and at WARN level
// for calls that return a non-OK status. Attributes included:
//   - "grpc.method"     — full gRPC method path.
//   - "grpc.duration"   — total handler duration as a time.Duration string.
//   - "grpc.code"       — gRPC status code name (e.g. "OK", "NotFound").
//   - "grpc.error"      — error message, present only on non-OK responses.
//
// Purpose: Provides baseline observability for every unary RPC without
// requiring callers to add per-handler logging boilerplate.
// Constraints: Positioned after RecoveryUnaryInterceptor in the chain so that
// panics are already translated into proper gRPC errors before this interceptor
// records the final status code.
// Thread-safety: The returned interceptor is safe for concurrent invocation
// across thousands of simultaneous RPCs.
func LoggingUnaryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		// We extract the underlying gRPC status code to emit structured logs.
		// This allows monitoring tools to easily aggregate metrics based on
		// specific RPC success or failure outcomes.
		code := status.Code(err)
		attrs := []any{
			slog.String("grpc.method", info.FullMethod),
			slog.String("grpc.duration", duration.String()),
			slog.String("grpc.code", code.String()),
		}
		if err != nil {
			attrs = append(attrs, slog.String("grpc.error", err.Error()))
			logger.WarnContext(ctx, "grpc: unary call finished with error", attrs...)
		} else {
			logger.InfoContext(ctx, "grpc: unary call finished", attrs...)
		}
		return resp, err
	}
}

// LoggingStreamInterceptor returns a gRPC stream server interceptor that emits
// a single structured log entry per streaming RPC, capturing the full method
// path, wall-clock duration of the entire stream lifetime, and the final gRPC
// status code.
//
// The log record mirrors the structure of LoggingUnaryInterceptor with
// "grpc.method", "grpc.duration", "grpc.code", and optional "grpc.error".
//
// Purpose: Provides baseline observability for every streaming RPC without
// per-handler boilerplate.
// Constraints: Positioned after RecoveryStreamInterceptor in the chain so that
// panics are translated before the status code is recorded.
// Thread-safety: The returned interceptor is safe for concurrent invocation
// across simultaneous streaming RPCs.
func LoggingStreamInterceptor(logger *slog.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)

		// Extracting the final status code from the streaming error output.
		// By logging this explicitly as an attribute, we maintain consistency
		// with the unary log format for seamless observability integration.
		code := status.Code(err)
		attrs := []any{
			slog.String("grpc.method", info.FullMethod),
			slog.String("grpc.duration", duration.String()),
			slog.String("grpc.code", code.String()),
		}
		if err != nil {
			attrs = append(attrs, slog.String("grpc.error", err.Error()))
			logger.WarnContext(ss.Context(), "grpc: stream call finished with error", attrs...)
		} else {
			logger.InfoContext(ss.Context(), "grpc: stream call finished", attrs...)
		}
		return err
	}
}
