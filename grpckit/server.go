package grpckit

import (
	"crypto/tls"
	"log/slog"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// serverConfig models the complete, immutable set of parameters that govern
// how the gRPC server is bootstrapped, including its network address, TLS
// posture, and middleware chain.
// Purpose: Aggregates all server-side configuration in a single value that is
// fully resolved before the server is started.
// Constraints: All fields are read-only after the constructor completes, making
// the configuration safe to inspect from any goroutine.
// Thread-safety: All fields are read-only after initialization and thus thread-safe.
type serverConfig struct {
	// addr is the TCP address to listen on, e.g. ":50051".
	addr string
	// tlsConfig is the optional TLS configuration. When nil the server uses
	// the insecure transport (suitable for internal service meshes only).
	tlsConfig *tls.Config
	// logger is the structured logger used by the built-in interceptors.
	logger *slog.Logger
	// unaryInterceptors contains caller-supplied unary server interceptors
	// that are chained after the built-in recovery and logging interceptors.
	unaryInterceptors []grpc.UnaryServerInterceptor
	// streamInterceptors contains caller-supplied stream server interceptors
	// that are chained after the built-in recovery and logging interceptors.
	streamInterceptors []grpc.StreamServerInterceptor
	// dialTimeout is the maximum time NewServer will block waiting for the
	// server to be ready. Zero means no explicit server-side timeout.
	dialTimeout time.Duration
}

// defaultServerConfig returns a serverConfig pre-populated with production-safe
// defaults: port 50051, no TLS, the default slog logger.
func defaultServerConfig() serverConfig {
	return serverConfig{
		addr:        ":50051",
		logger:      slog.Default(),
		dialTimeout: 10 * time.Second,
	}
}

// ServerOption establishes a typed functional parameter contract, affording
// consumers the capability to sequentially override targeted properties inside
// the serverConfig before the server is constructed.
// Purpose: Enables the Functional Options pattern for gRPC server configuration.
// Constraints: Options are evaluated serially during construction. Nil options
// are silently skipped to prevent panics.
// Thread-safety: Safe when used sequentially during initialization.
type ServerOption func(*serverConfig)

// WithServerAddress sets the TCP address on which the gRPC server will accept
// connections (e.g. ":50051" or "0.0.0.0:8080").
// Purpose: Overrides the default listen address ":50051".
// Constraints: The address must be a valid TCP host:port string. An empty value
// falls back to the system-chosen port.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithServerAddress(addr string) ServerOption {
	return func(c *serverConfig) {
		c.addr = addr
	}
}

// WithServerTLS configures the server to terminate TLS using the supplied
// *tls.Config. When this option is omitted the server uses the gRPC insecure
// transport, which is only appropriate inside trusted internal networks.
// Purpose: Enables mutual or one-way TLS on the gRPC listener.
// Constraints: cfg must not be nil; pass a properly configured *tls.Config
// loaded from your certificate files.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithServerTLS(cfg *tls.Config) ServerOption {
	return func(c *serverConfig) {
		if cfg != nil {
			c.tlsConfig = cfg
		}
	}
}

// WithServerLogger replaces the slog.Logger used by the built-in logging and
// recovery interceptors. By default the package uses slog.Default().
// Purpose: Integrates grpckit into an application-wide structured logging setup.
// Constraints: logger must not be nil. A nil value is silently ignored and
// the existing logger is preserved.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithServerLogger(logger *slog.Logger) ServerOption {
	return func(c *serverConfig) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithUnaryInterceptors appends one or more additional unary server interceptors
// to the chain. They are executed after the built-in recovery and logging
// interceptors, so panics and log entries are always guaranteed.
// Purpose: Allows callers to extend the middleware chain with custom logic
// such as authentication, tracing, or metrics.
// Constraints: Nil interceptors in the variadic slice are silently skipped.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) ServerOption {
	return func(c *serverConfig) {
		// Filter out nil interceptors gracefully, ensuring the downstream
		// middleware execution pipeline does not panic due to empty function pointers.
		for _, i := range interceptors {
			if i != nil {
				c.unaryInterceptors = append(c.unaryInterceptors, i)
			}
		}
	}
}

// WithStreamInterceptors appends one or more additional stream server interceptors
// to the chain, executed after the built-in recovery and logging interceptors.
// Purpose: Allows callers to extend the streaming middleware chain with custom logic.
// Constraints: Nil interceptors in the variadic slice are silently skipped.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithStreamInterceptors(interceptors ...grpc.StreamServerInterceptor) ServerOption {
	return func(c *serverConfig) {
		// Like unary interceptors, we filter out nils to maintain safety
		// in the ordered stream middleware chain.
		for _, i := range interceptors {
			if i != nil {
				c.streamInterceptors = append(c.streamInterceptors, i)
			}
		}
	}
}

// parseServerOptions initialises a defaultServerConfig and sequentially applies
// every non-nil ServerOption. It is the single entry point for config resolution
// inside NewServer.
// Purpose: Aggregates modular setup logic into a validated serverConfig.
// Constraints: Should only be called internally during server construction.
// Thread-safety: Synchronous and safe.
func parseServerOptions(opts ...ServerOption) serverConfig {
	cfg := defaultServerConfig()
	// Safely evaluate all functional options in order, discarding nils
	// to prevent configuration panics during application bootstrap.
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

// NewServer constructs a fully configured *grpc.Server with a mandatory, built-in
// interceptor chain that guarantees panic recovery and structured request logging
// on every call, regardless of additional caller-supplied interceptors.
//
// The interceptor execution order for unary calls is:
//  1. RecoveryUnaryInterceptor  — catches panics; logs + returns codes.Internal.
//  2. LoggingUnaryInterceptor   — logs method, duration, and final status code.
//  3. Caller-supplied interceptors (via WithUnaryInterceptors), in order.
//
// Streaming calls follow the same order via their stream equivalents.
// Purpose: Bootstraps a production-ready gRPC server that never crashes due to
// unhandled panics and always emits structured observability data.
// Constraints: Returns a *grpc.Server that is not yet bound to a listener; the
// caller is responsible for calling Serve(net.Listener) on the returned value.
// Thread-safety: Construction is synchronous. The returned *grpc.Server is safe
// for concurrent use via its exported API.
func NewServer(opts ...ServerOption) *grpc.Server {
	cfg := parseServerOptions(opts...)

	// Build the mandatory unary chain: recovery → logging → user interceptors.
	// Internal Logic Deep-Dive: We rigidly enforce the interceptor sequence. The `RecoveryUnaryInterceptor` MUST be placed first so that it acts as the outermost shell, capable of catching panics originating from any subsequent interceptor or handler. `LoggingUnaryInterceptor` is placed second to ensure it logs the final gRPC code correctly, even if the error was generated by the recovery interceptor converting a panic. User interceptors are appended last.
	unaryChain := []grpc.UnaryServerInterceptor{
		RecoveryUnaryInterceptor(cfg.logger),
		LoggingUnaryInterceptor(cfg.logger),
	}
	unaryChain = append(unaryChain, cfg.unaryInterceptors...)

	// Build the mandatory stream chain: recovery → logging → user interceptors.
	// Internal Logic Deep-Dive: Similar to the unary chain, we enforce the order: Recovery -> Logging -> User Interceptors. This guarantees that stream handling remains fully observable and crash-resilient regardless of what third-party or application-level stream interceptors are injected.
	streamChain := []grpc.StreamServerInterceptor{
		RecoveryStreamInterceptor(cfg.logger),
		LoggingStreamInterceptor(cfg.logger),
	}
	streamChain = append(streamChain, cfg.streamInterceptors...)

	grpcOpts := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(unaryChain...),
		grpc.ChainStreamInterceptor(streamChain...),
	}

	// Attach transport credentials only when a TLS config was provided;
	// otherwise fall back to the insecure transport.
	if cfg.tlsConfig != nil {
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(cfg.tlsConfig)))
	} else {
		grpcOpts = append(grpcOpts, grpc.Creds(insecure.NewCredentials()))
	}

	return grpc.NewServer(grpcOpts...)
}
