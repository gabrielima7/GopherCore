package grpckit

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestClientOptions(t *testing.T) {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	unaryInt := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	streamInt := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(ctx, desc, cc, method, opts...)
	}
	rawDialOpt := grpc.WithBlock()

	tests := []struct {
		name     string
		opts     []ClientOption
		validate func(t *testing.T, cfg clientConfig)
	}{
		{
			name: "no options provided",
			opts: nil,
			validate: func(t *testing.T, cfg clientConfig) {
				if cfg.dialTimeout != 10*time.Second {
					t.Errorf("expected 10s default timeout, got %v", cfg.dialTimeout)
				}
				if cfg.tlsConfig != nil {
					t.Errorf("expected nil tlsConfig, got %v", cfg.tlsConfig)
				}
			},
		},
		{
			name: "slice with nil options safety",
			opts: []ClientOption{nil, WithDialTimeout(5 * time.Second), nil},
			validate: func(t *testing.T, cfg clientConfig) {
				if cfg.dialTimeout != 5*time.Second {
					t.Errorf("expected 5s timeout, got %v", cfg.dialTimeout)
				}
			},
		},
		{
			name: "WithInsecure",
			opts: []ClientOption{WithClientTLS(tlsCfg), WithInsecure()},
			validate: func(t *testing.T, cfg clientConfig) {
				if cfg.tlsConfig != nil {
					t.Errorf("expected nil tlsConfig after WithInsecure, got %v", cfg.tlsConfig)
				}
			},
		},
		{
			name: "WithClientTLS",
			opts: []ClientOption{WithClientTLS(tlsCfg)},
			validate: func(t *testing.T, cfg clientConfig) {
				if cfg.tlsConfig != tlsCfg {
					t.Errorf("expected custom tls config, got %v", cfg.tlsConfig)
				}
			},
		},
		{
			name: "WithClientTLS nil safety",
			opts: []ClientOption{WithClientTLS(tlsCfg), WithClientTLS(nil)},
			validate: func(t *testing.T, cfg clientConfig) {
				if cfg.tlsConfig != tlsCfg {
					t.Errorf("expected tls config to remain intact after nil, got %v", cfg.tlsConfig)
				}
			},
		},
		{
			name: "WithDialTimeout valid",
			opts: []ClientOption{WithDialTimeout(3 * time.Second)},
			validate: func(t *testing.T, cfg clientConfig) {
				if cfg.dialTimeout != 3*time.Second {
					t.Errorf("expected 3s timeout, got %v", cfg.dialTimeout)
				}
			},
		},
		{
			name: "WithDialTimeout zero or negative safety",
			opts: []ClientOption{WithDialTimeout(0), WithDialTimeout(-1)},
			validate: func(t *testing.T, cfg clientConfig) {
				if cfg.dialTimeout != 10*time.Second {
					t.Errorf("expected 10s default timeout, got %v", cfg.dialTimeout)
				}
			},
		},
		{
			name: "WithClientUnaryInterceptors",
			opts: []ClientOption{WithClientUnaryInterceptors(nil, unaryInt, nil)},
			validate: func(t *testing.T, cfg clientConfig) {
				if len(cfg.unaryInterceptors) != 1 {
					t.Errorf("expected 1 unary interceptor, got %d", len(cfg.unaryInterceptors))
				}
			},
		},
		{
			name: "WithClientStreamInterceptors",
			opts: []ClientOption{WithClientStreamInterceptors(nil, streamInt, nil)},
			validate: func(t *testing.T, cfg clientConfig) {
				if len(cfg.streamInterceptors) != 1 {
					t.Errorf("expected 1 stream interceptor, got %d", len(cfg.streamInterceptors))
				}
			},
		},
		{
			name: "WithRawDialOptions",
			opts: []ClientOption{WithRawDialOptions(nil, rawDialOpt, nil)},
			validate: func(t *testing.T, cfg clientConfig) {
				if len(cfg.rawDialOpts) != 1 {
					t.Errorf("expected 1 raw dial option, got %d", len(cfg.rawDialOpts))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := parseClientOptions(tt.opts...)
			tt.validate(t, cfg)
		})
	}
}

func TestNewClient(t *testing.T) {
	// Test pure option configurations without network dependencies by using an explicitly invalid target.
	// As per instructions, avoid invoking the actual dialer with a real target if we only want to test options.
	// grpc.DialContext parses the target and can fail fast if it's completely invalid.

	conn, err := NewClient("127.0.0.1:1", WithDialTimeout(10*time.Millisecond))

	// Since we are not passing grpc.WithBlock(), the Dial might succeed in creating the ClientConn
	// even if the target is unreachable (it connects in the background).
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	defer conn.Close()

	// A basic test just to ensure NewClient executes the option parsing and returns a client
}
