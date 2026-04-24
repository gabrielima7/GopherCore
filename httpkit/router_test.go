package httpkit

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func mustCloseRouterTest(t *testing.T, closer interface{ Close() error }) {
	t.Helper()
	if err := closer.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("close failed: %v", err)
	}
}

func TestRouterConfigurationOptions(t *testing.T) {
	tests := []struct {
		name     string
		opts     []RouterOption
		validate func(t *testing.T, cfg RouterConfig)
	}{
		{
			name: "Default configuration",
			opts: nil,
			validate: func(t *testing.T, cfg RouterConfig) {
				if cfg.RateLimit != 100 {
					t.Errorf("expected RateLimit 100, got %f", cfg.RateLimit)
				}
				if cfg.RateBurst != 200 {
					t.Errorf("expected RateBurst 200, got %d", cfg.RateBurst)
				}
				if cfg.ReadTimeout != 15*time.Second {
					t.Errorf("expected ReadTimeout 15s, got %v", cfg.ReadTimeout)
				}
				if !cfg.EnableLogger {
					t.Errorf("expected EnableLogger true, got false")
				}
			},
		},
		{
			name: "With nil option explicitly passed",
			opts: []RouterOption{nil},
			validate: func(t *testing.T, cfg RouterConfig) {
				// Should just safely ignore the nil option and keep defaults
				if cfg.RateLimit != 100 {
					t.Errorf("expected RateLimit 100, got %f", cfg.RateLimit)
				}
			},
		},
		{
			name: "With CORS",
			opts: []RouterOption{WithCORS("https://example.com")},
			validate: func(t *testing.T, cfg RouterConfig) {
				if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "https://example.com" {
					t.Errorf("expected AllowedOrigins [https://example.com], got %v", cfg.AllowedOrigins)
				}
			},
		},
		{
			name: "With zero rate limit and burst",
			opts: []RouterOption{WithRateLimit(0, 0)},
			validate: func(t *testing.T, cfg RouterConfig) {
				if cfg.RateLimit != 0 {
					t.Errorf("expected RateLimit 0, got %f", cfg.RateLimit)
				}
				if cfg.RateBurst != 0 {
					t.Errorf("expected RateBurst 0, got %d", cfg.RateBurst)
				}
			},
		},
		{
			name: "With Read/Write Timeouts",
			opts: []RouterOption{
				WithReadTimeout(10 * time.Second),
				WithReadHeaderTimeout(2 * time.Second),
				WithWriteTimeout(20 * time.Second),
			},
			validate: func(t *testing.T, cfg RouterConfig) {
				if cfg.ReadTimeout != 10*time.Second {
					t.Errorf("expected ReadTimeout 10s, got %v", cfg.ReadTimeout)
				}
				if cfg.ReadHeaderTimeout != 2*time.Second {
					t.Errorf("expected ReadHeaderTimeout 2s, got %v", cfg.ReadHeaderTimeout)
				}
				if cfg.WriteTimeout != 20*time.Second {
					t.Errorf("expected WriteTimeout 20s, got %v", cfg.WriteTimeout)
				}
			},
		},
		{
			name: "With Logger disabled",
			opts: []RouterOption{WithLogger(false)},
			validate: func(t *testing.T, cfg RouterConfig) {
				if cfg.EnableLogger {
					t.Errorf("expected EnableLogger false, got true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := parseOptions(tt.opts...)
			tt.validate(t, cfg)
		})
	}
}

func TestNewRouterIntegration(t *testing.T) {
	tests := []struct {
		name         string
		opts         []RouterOption
		reqMethod    string
		reqPath      string
		reqOrigin    string
		expectedCode int
	}{
		{
			name:         "Basic GET request",
			opts:         []RouterOption{WithLogger(false)},
			reqMethod:    http.MethodGet,
			reqPath:      "/test",
			expectedCode: http.StatusOK,
		},
		{
			name:         "CORS allowed origin",
			opts:         []RouterOption{WithCORS("https://example.com"), WithLogger(false)},
			reqMethod:    http.MethodGet,
			reqPath:      "/test",
			reqOrigin:    "https://example.com",
			expectedCode: http.StatusOK,
		},
		{
			name: "RateLimit burst zero defaults to limit",
			opts: []RouterOption{WithRateLimit(10, 0), WithLogger(false)}, // RateBurst will be set to int(10) inside NewRouter
			reqMethod:    http.MethodGet,
			reqPath:      "/test",
			expectedCode: http.StatusOK,
		},
		{
			name: "RateLimit disabled",
			opts: []RouterOption{WithRateLimit(0, 0), WithLogger(false)},
			reqMethod:    http.MethodGet,
			reqPath:      "/test",
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRouter(tt.opts...)
			if r == nil {
				t.Fatal("expected non-nil router")
			}

			r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(tt.reqMethod, tt.reqPath, nil)
			if tt.reqOrigin != "" {
				req.Header.Set("Origin", tt.reqOrigin)
			}

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedCode {
				t.Errorf("expected code %d, got %d", tt.expectedCode, rr.Code)
			}
		})
	}
}

func TestNewServer(t *testing.T) {
	r := NewRouter(WithLogger(false))
	srv := NewServer(":8080", r, WithReadTimeout(5*time.Second), WithReadHeaderTimeout(2*time.Second), WithWriteTimeout(5*time.Second))
	if srv.Addr != ":8080" {
		t.Fatalf("expected :8080, got %s", srv.Addr)
	}
	if srv.ReadTimeout != 5*time.Second {
		t.Fatalf("expected 5s read timeout, got %v", srv.ReadTimeout)
	}
	if srv.ReadHeaderTimeout != 2*time.Second {
		t.Fatalf("expected 2s read header timeout, got %v", srv.ReadHeaderTimeout)
	}
	if srv.WriteTimeout != 5*time.Second {
		t.Fatalf("expected 5s write timeout, got %v", srv.WriteTimeout)
	}
}

func TestGracefulShutdown(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		setupServer func(t *testing.T) (*http.Server, func())
		triggerWait func(t *testing.T)
		wantErr     bool
	}{
		{
			name:    "Server natively closed returns ErrServerClosed handled as nil",
			timeout: 5 * time.Second,
			setupServer: func(t *testing.T) (*http.Server, func()) {
				srv := &http.Server{
					Addr:              "127.0.0.1:0",
					Handler:           http.DefaultServeMux,
					ReadHeaderTimeout: 5 * time.Second,
				}
				return srv, func() {} // No cleanup needed
			},
			triggerWait: func(t *testing.T) {
				time.Sleep(50 * time.Millisecond) // Let server start
			},
			wantErr: false,
		},
		{
			name:    "Server port already in use returns error",
			timeout: 5 * time.Second,
			setupServer: func(t *testing.T) (*http.Server, func()) {
				// Start dummy server to occupy port
				dummy := &http.Server{Addr: "127.0.0.1:0", ReadHeaderTimeout: 5 * time.Second}
				ln, err := net.Listen("tcp", dummy.Addr)
				if err != nil {
					t.Fatalf("failed to listen: %v", err)
				}

				go func() {
					if serveErr := dummy.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
						t.Errorf("dummy server failed: %v", serveErr)
					}
				}()

				// Return server configured for the SAME address
				srv := &http.Server{
					Addr:              ln.Addr().String(),
					Handler:           http.DefaultServeMux,
					ReadHeaderTimeout: 5 * time.Second,
				}

				cleanup := func() {
					dummy.Shutdown(context.Background())
					mustCloseRouterTest(t, ln)
				}

				return srv, cleanup
			},
			triggerWait: func(t *testing.T) {
				// Nothing needed, should fail immediately on listen
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, cleanup := tt.setupServer(t)
			defer cleanup()

			errCh := make(chan error, 1)
			go func() {
				errCh <- GracefulShutdown(srv, tt.timeout)
			}()

			tt.triggerWait(t)

			// For the successful path test, explicitly close the server
			if !tt.wantErr {
				if err := srv.Close(); err != nil {
					t.Fatalf("failed to close server: %v", err)
				}
			}

			select {
			case err := <-errCh:
				if (err != nil) != tt.wantErr {
					t.Errorf("GracefulShutdown() error = %v, wantErr %v", err, tt.wantErr)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for GracefulShutdown")
			}
		})
	}
}

func TestGracefulShutdown_Signal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Signal not supported on Windows")
	}

	srv := &http.Server{
		Addr: "127.0.0.1:0", // Listen on any available port
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond) // Simulate work
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- GracefulShutdown(srv, 5*time.Second)
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Send SIGINT
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("failed to find process: %v", err)
	}
	if err := p.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// Wait for GracefulShutdown to return
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for GracefulShutdown to return")
	}
}

func TestNewRouterWithLogger(t *testing.T) {
	r := NewRouter(WithLogger(true))
	r.Get("/logger", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/logger", nil)
	rr := httptest.NewRecorder()

	// This will hit the middleware.Logger code path
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rr.Code)
	}
}
