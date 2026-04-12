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

// RouterConfig holds configuration options for the router.
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
	// WriteTimeout for the HTTP server.
	WriteTimeout time.Duration
	// EnableLogger enables the chi request logger middleware.
	EnableLogger bool
}

// DefaultRouterConfig returns a sensible default configuration.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		RateLimit:      100,
		RateBurst:      200,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		EnableLogger:   true,
	}
}

// RouterOption is a functional option for configuring the router.
type RouterOption func(*RouterConfig)

// WithCORS sets the allowed CORS origins.
func WithCORS(origins ...string) RouterOption {
	return func(c *RouterConfig) {
		c.AllowedOrigins = origins
	}
}

// WithRateLimit sets the rate limit (requests per second) and burst size.
func WithRateLimit(rps float64, burst int) RouterOption {
	return func(c *RouterConfig) {
		c.RateLimit = rps
		c.RateBurst = burst
	}
}

// WithReadTimeout sets the HTTP server read timeout.
func WithReadTimeout(d time.Duration) RouterOption {
	return func(c *RouterConfig) {
		c.ReadTimeout = d
	}
}

// WithWriteTimeout sets the HTTP server write timeout.
func WithWriteTimeout(d time.Duration) RouterOption {
	return func(c *RouterConfig) {
		c.WriteTimeout = d
	}
}

// WithLogger enables or disables the request logger.
func WithLogger(enabled bool) RouterOption {
	return func(c *RouterConfig) {
		c.EnableLogger = enabled
	}
}

// NewRouter creates a new chi.Mux with security middleware pre-configured.
// It includes: RequestID, RealIP, Recoverer, SecurityHeaders, and optionally
// Logger, RateLimit, and CORS based on configuration.
func NewRouter(opts ...RouterOption) *chi.Mux {
	cfg := DefaultRouterConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

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

// NewServer creates an http.Server with the given router and configuration timeouts.
func NewServer(addr string, handler http.Handler, opts ...RouterOption) *http.Server {
	cfg := DefaultRouterConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
}

// GracefulShutdown intercepts OS signals (SIGINT, SIGTERM) and performs a graceful
// shutdown of the server using server.Shutdown. It waits up to the specified
// timeout for ongoing requests to complete.
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
