package grpckit

import (
	"context"
	"crypto/tls"
	"go.uber.org/goleak"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestClientOptions(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("google.golang.org/grpc/internal/grpcsync.(*CallbackSerializer).run"))
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	unaryInt := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	streamInt := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(ctx, desc, cc, method, opts...)
	}
	rawDialOpt := grpc.WithAuthority("test")

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
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("google.golang.org/grpc/internal/grpcsync.(*CallbackSerializer).run"))
	// Happy path: start a local gRPC server on an ephemeral port to provide a real target for the dialer.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	go func() {
		_ = srv.Serve(lis)
	}()
	defer srv.Stop()

	// Connect to the real local server using insecure transport and a custom user agent option.
	conn, err := NewClient(lis.Addr().String(), WithDialTimeout(2*time.Second), WithInsecure(), WithRawDialOptions(grpc.WithAuthority("test")))
	if err != nil {
		t.Fatalf("expected successful connection, got error: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if conn == nil {
		t.Fatal("expected non-nil connection object")
	}
}

func TestNewClient_ErrorPath(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("google.golang.org/grpc/internal/grpcsync.(*CallbackSerializer).run"))
	tests := []struct {
		name    string
		target  string
		opts    []ClientOption
		wantErr bool
	}{
		{
			name:    "invalid target with control character",
			target:  "\x00",
			opts:    []ClientOption{WithInsecure()},
			wantErr: true,
		},
		{
			name:    "empty target",
			target:  "",
			opts:    []ClientOption{WithInsecure()},
			wantErr: false, // grpc.NewClient allows empty target
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := NewClient(tt.target, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && conn != nil {
				t.Errorf("expected nil connection on error, got %v", conn)
			}
			if conn != nil {
				_ = conn.Close()
			}
		})
	}
}
