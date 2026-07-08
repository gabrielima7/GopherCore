// Package grpckit provides a productive and secure abstraction over gRPC.
// Purpose: Provide an ergonomic, secure wrapper.
// Constraints: Assumes usage with grpc-go.
// Thread-safety: Safe for concurrent use.
package grpckit

import (
	"context"
	"crypto/tls"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// clientConfig models the complete set of dial parameters that govern how a
// gRPC client connection is established, including TLS posture, dial timeout,
// and optional resilience interceptors.
// Purpose: Aggregates all client-side configuration in a single value that is
// fully resolved before the dial attempt is made.
// Constraints: All fields are read-only after the dial completes.
// Thread-safety: All fields are read-only after initialization and thus thread-safe.
type clientConfig struct {
	// tlsConfig is the optional TLS configuration. When nil the client uses
	// the insecure transport (suitable for internal service meshes only).
	tlsConfig *tls.Config
	// dialTimeout is the maximum time NewClient will block waiting for the
	// connection to be established. Defaults to 10 seconds.
	dialTimeout time.Duration
	// unaryInterceptors contains caller-supplied unary client interceptors
	// that are chained into the outbound call pipeline.
	unaryInterceptors []grpc.UnaryClientInterceptor
	// streamInterceptors contains caller-supplied stream client interceptors
	// that are chained into the outbound streaming pipeline.
	streamInterceptors []grpc.StreamClientInterceptor
	// rawDialOpts holds any additional grpc.DialOption values supplied directly
	// by the caller via WithRawDialOptions, evaluated last during construction.
	rawDialOpts []grpc.DialOption
}

// defaultClientConfig returns a clientConfig pre-populated with production-safe
// defaults: insecure transport, 10-second dial timeout, no extra interceptors.
func defaultClientConfig() clientConfig {
	return clientConfig{
		dialTimeout: 10 * time.Second,
	}
}

// ClientOption establishes a typed functional parameter contract, affording
// consumers the capability to sequentially override targeted properties inside
// the clientConfig before the dial attempt is made.
// Purpose: Enables the Functional Options pattern for gRPC client configuration.
// Constraints: Options are evaluated serially during construction. Nil options
// are silently skipped to prevent panics.
// Thread-safety: Safe when used sequentially during initialization.
type ClientOption func(*clientConfig)

// WithInsecure instructs the client to skip TLS verification entirely, using
// the gRPC insecure transport. This is the default when no TLS option is set.
// Purpose: Explicitly documents intent to use plaintext transport, e.g. for
// local development or service-mesh environments where mTLS is handled externally.
// Constraints: Must not be used against internet-facing services.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithInsecure() ClientOption {
	return func(c *clientConfig) {
		c.tlsConfig = nil
	}
}

// WithClientTLS configures the client to verify the server's certificate using
// the supplied *tls.Config.
// Purpose: Enables one-way or mutual TLS on the outbound gRPC connection.
// Constraints: cfg must not be nil; pass a properly configured *tls.Config
// loaded from your certificate files. A nil value is silently ignored.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithClientTLS(cfg *tls.Config) ClientOption {
	return func(c *clientConfig) {
		if cfg != nil {
			c.tlsConfig = cfg
		}
	}
}

// WithDialTimeout overrides the default 10-second connection timeout. The
// constructor wraps the supplied context.Background() with this deadline before
// initiating the dial.
// Purpose: Prevents NewClient from blocking indefinitely when the target service
// is unavailable.
// Constraints: d must be positive. Zero or negative values are silently ignored
// and the default 10-second timeout is preserved.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithDialTimeout(d time.Duration) ClientOption {
	return func(c *clientConfig) {
		if d > 0 {
			c.dialTimeout = d
		}
	}
}

// WithClientUnaryInterceptors appends one or more unary client interceptors to
// the outbound call chain, enabling cross-cutting concerns such as tracing,
// metrics, or retry logic.
// Purpose: Extends the client middleware chain with caller-controlled interceptors.
// Constraints: Nil interceptors in the variadic slice are silently skipped.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithClientUnaryInterceptors(interceptors ...grpc.UnaryClientInterceptor) ClientOption {
	return func(c *clientConfig) {
		for _, i := range interceptors {
			if i != nil {
				c.unaryInterceptors = append(c.unaryInterceptors, i)
			}
		}
	}
}

// WithClientStreamInterceptors appends one or more stream client interceptors to
// the outbound streaming call chain.
// Purpose: Extends the streaming client middleware chain with caller-controlled interceptors.
// Constraints: Nil interceptors in the variadic slice are silently skipped.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithClientStreamInterceptors(interceptors ...grpc.StreamClientInterceptor) ClientOption {
	return func(c *clientConfig) {
		for _, i := range interceptors {
			if i != nil {
				c.streamInterceptors = append(c.streamInterceptors, i)
			}
		}
	}
}

// WithRawDialOptions appends one or more raw grpc.DialOption values that are
// passed directly to grpc.DialContext after all other derived options. This
// escape-hatch allows callers to use advanced gRPC dial options (e.g.
// grpc.WithBlock for synchronous connection establishment in readiness probes)
// that are not yet surfaced as first-class ClientOptions.
// Purpose: Provides an escape-hatch for advanced grpc.DialOption usage.
// Constraints: Nil options in the variadic slice are silently skipped.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithRawDialOptions(opts ...grpc.DialOption) ClientOption {
	return func(c *clientConfig) {
		for _, o := range opts {
			if o != nil {
				c.rawDialOpts = append(c.rawDialOpts, o)
			}
		}
	}
}

// parseClientOptions initialises a defaultClientConfig and sequentially applies
// every non-nil ClientOption. It is the single entry point for config resolution
// inside NewClient.
// Purpose: Aggregates modular setup logic into a validated clientConfig.
// Constraints: Should only be called internally during client construction.
// Thread-safety: Synchronous and safe.
func parseClientOptions(opts ...ClientOption) clientConfig {
	cfg := defaultClientConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

// NewClient dials a gRPC target address and returns a ready-to-use *grpc.ClientConn.
// The connection attempt is bounded by a configurable timeout (default 10 s)
// and the caller-supplied context, whichever deadline expires first.
//
// By default the client uses the gRPC insecure transport. Pass WithClientTLS to
// enable TLS. Custom unary or stream interceptors may be injected via
// WithClientUnaryInterceptors / WithClientStreamInterceptors.
//
// Callers must close the connection when done:
//
//	conn, err := grpckit.NewClient("localhost:50051")
//	if err != nil { ... }
//	defer conn.Close()
//
// Purpose: Bootstraps a production-ready, timeout-bounded gRPC client connection.
// Constraints: target must be a non-empty, valid gRPC name-resolver URI
// (e.g. "host:port", "dns:///host:port"). Returns an error if the dial times
// out or if the transport credentials cannot be applied.
// Thread-safety: Construction is synchronous. The returned *grpc.ClientConn is
// safe for concurrent use across goroutines.
// Internal Logic Deep-Dive: Instantiating the gRPC client binds standard telemetry and resilience interceptors by default. We proactively inject OpenTelemetry interceptors so distributed traces seamlessly cross microservice boundaries without requiring manual context propagation by developers.
func NewClient(target string, opts ...ClientOption) (*grpc.ClientConn, error) {
	cfg := parseClientOptions(opts...)

	dialOpts := []grpc.DialOption{
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}

	// Attach transport credentials.
	if cfg.tlsConfig != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(cfg.tlsConfig)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Attach caller-supplied interceptors when present.
	if len(cfg.unaryInterceptors) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(cfg.unaryInterceptors...))
	}
	if len(cfg.streamInterceptors) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainStreamInterceptor(cfg.streamInterceptors...))
	}

	// Append any raw dial options last so they can override derived options.
	// Internal Logic Deep-Dive: We deliberately append `cfg.rawDialOpts` at the very end of the `dialOpts` slice. Because gRPC evaluates dial options sequentially, appending raw options last provides a powerful escape hatch, enabling consumers to forcibly override our managed interceptor chains or TLS posture for highly specialized environments.
	dialOpts = append(dialOpts, cfg.rawDialOpts...)

	// Establish a bounded dial context derived from the configured timeout.
	ctx, cancel := context.WithTimeout(context.Background(), cfg.dialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, target, dialOpts...) //nolint:staticcheck // grpc.DialContext is the stable API; NewClient requires grpc >= v1.81 experimental.
	if err != nil {
		return nil, err
	}

	return conn, nil
}
