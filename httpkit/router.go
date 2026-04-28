// Package httpkit provides an HTTP toolkit built on go-chi/chi with
// pre-configured security middleware, rate limiting, CORS control,
// and standardized JSON responses.
package httpkit

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"
)

// RouterConfig holds configuration options for building and mounting the core
// application router, including CORS, rate limiting, and HTTP timeouts.
// All fields are read-only after initialization and thus thread-safe.
type RouterConfig struct {
	// AllowedOrigins for CORS. Empty means no CORS middleware.
	AllowedOrigins []string
	// AllowedMethods for CORS. Defaults to GET, POST, PUT, DELETE, OPTIONS.
	AllowedMethods []string
	// AllowedHeaders for CORS. Defaults to Accept, Authorization, Content-Type.
	AllowedHeaders []string
	// RateLimit is the maximum requests per second. 0 disables rate limiting.
	RateLimit float64
	// RateBurst is the maximum burst size for rate limiting. Defaults to RateLimit.
	RateBurst int
	// ReadTimeout for the HTTP server.
	ReadTimeout time.Duration
	// ReadHeaderTimeout for the HTTP server.
	ReadHeaderTimeout time.Duration
	// WriteTimeout for the HTTP server.
	WriteTimeout time.Duration
	// IdleTimeout for the HTTP server.
	IdleTimeout time.Duration
	// EnableLogger enables the chi request logger middleware.
	EnableLogger bool
}

// DefaultRouterConfig returns a secure and sensible baseline configuration
// for the HTTP router to mitigate standard application vulnerabilities natively.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		AllowedMethods:    []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:    []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		RateLimit:         100,
		RateBurst:         200,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       120 * time.Second,
		EnableLogger:      true,
	}
}

// RouterOption defines a functional option signature for configuring the router instance
// mutatively during setup.
type RouterOption func(*RouterConfig)

// WithCORS restricts the Cross-Origin Resource Sharing policy to only accept
// preflight and incoming requests from the strictly allowed array of origins.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithCORS(origins ...string) RouterOption {
	return func(c *RouterConfig) {
		c.AllowedOrigins = origins
	}
}

// WithRateLimit configures global inbound traffic limits by specifying the allowed
// requests per second (rps) and a maximum concurrent burst size.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithRateLimit(rps float64, burst int) RouterOption {
	return func(c *RouterConfig) {
		c.RateLimit = rps
		c.RateBurst = burst
	}
}

// WithReadTimeout strictly enforces the maximum duration the server will wait
// while reading the full client HTTP request headers and body payload.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithReadTimeout(d time.Duration) RouterOption {
	return func(c *RouterConfig) {
		c.ReadTimeout = d
	}
}

// WithReadHeaderTimeout strictly enforces the maximum duration the server will wait
// while reading the HTTP request headers, helping to mitigate Slowloris-style attacks.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithReadHeaderTimeout(d time.Duration) RouterOption {
	return func(c *RouterConfig) {
		c.ReadHeaderTimeout = d
	}
}

// WithWriteTimeout strictly enforces the maximum duration the server is allowed
// to spend generating and writing the HTTP response back to the connected client.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithWriteTimeout(d time.Duration) RouterOption {
	return func(c *RouterConfig) {
		c.WriteTimeout = d
	}
}

// WithIdleTimeout strictly enforces the maximum duration the server is allowed
// to keep idle keep-alive connections open.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithIdleTimeout(d time.Duration) RouterOption {
	return func(c *RouterConfig) {
		c.IdleTimeout = d
	}
}

// WithLogger toggles the attachment of the chi structured HTTP request logging
// middleware on the internal Mux router.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
func WithLogger(enabled bool) RouterOption {
	return func(c *RouterConfig) {
		c.EnableLogger = enabled
	}
}

// parseOptions is an internal helper that initializes the DefaultRouterConfig
// and then safely applies all provided functional options.
// Purpose: Aggregates modular setup logic.
// Thread-safety: Synchronous and safe.
func parseOptions(opts ...RouterOption) RouterConfig {
	cfg := DefaultRouterConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

// NewRouter constructs and returns a highly-opinionated `chi.Mux` router
// bundled with a robust, pre-configured middleware stack.
//
// The default stack enforces request tracing (RequestID), client IP extraction (RealIP),
// panic safety (Recoverer), and strict security headers. Optional middlewares
// (Logger, RateLimit, CORS) are injected based on the provided options.
// Thread-safety: Safely initializes global middlewares for concurrent request processing.
func NewRouter(opts ...RouterOption) *chi.Mux {
	cfg := parseOptions(opts...)

	r := chi.NewRouter()

	// Core middleware stack.
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	if cfg.EnableLogger {
		r.Use(middleware.Logger)
	}

	// Security headers — always enabled.
	r.Use(SecurityHeadersMiddleware)

	// Rate limiting.
	if cfg.RateLimit > 0 {
		burst := cfg.RateBurst
		if burst <= 0 {
			burst = int(cfg.RateLimit)
		}
		limiter := rate.NewLimiter(rate.Limit(cfg.RateLimit), burst)
		r.Use(RateLimitMiddleware(limiter))
	}

	// CORS.
	if len(cfg.AllowedOrigins) > 0 {
		r.Use(CORSMiddleware(cfg.AllowedOrigins, cfg.AllowedMethods, cfg.AllowedHeaders))
	}

	return r
}

// NewServer creates and returns an `http.Server` bound to the provided network address.
//
// Purpose: It applies the read/write timeouts derived from the router options to prevent
// slowloris and other resource exhaustion attacks natively at the stdlib server level.
// Thread-safety: Initialization only; the underlying net/http handling is safely concurrent.
func NewServer(addr string, handler http.Handler, opts ...RouterOption) *http.Server {
	cfg := parseOptions(opts...)
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

// GracefulShutdown starts the HTTP server in a background goroutine and concurrently listens
// for OS termination signals (SIGINT, SIGTERM).
//
// Purpose: Upon receiving a termination signal, it invokes the server's Shutdown method, giving ongoing
// active requests up to the specified timeout duration to complete before forcing a closure.
// Thread-safety: It manages synchronization internally via channels and safely blocks the
// calling goroutine until shutdown completes or times out, safely orchestrating multiple concurrent signals.
func GracefulShutdown(srv *http.Server, timeout time.Duration) error {
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-quit:
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}
