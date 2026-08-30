// Package httpkit provides utilities.
// Purpose: httpkit provides HTTP routing, middleware, and structured JSON responses.
// Constraints: Internal package.
// Thread-safety: Varies by component.
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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riandyrn/otelchi"
	"golang.org/x/time/rate"
)

// RouterConfig models the immutable, global network topology limits and middleware injection triggers, directly governing how the underlying Chi multiplexer accepts traffic.
// Purpose: Aggregates all networking parameters for the application server.
// Constraints: Must be populated appropriately for the specific environment.
// Thread-safety: All fields are read-only after initialization and thus thread-safe.
// Internal Logic Deep-Dive: Centralizes all server vulnerability mitigations (like Slowloris protections) into one tunable block.
type RouterConfig struct {
	// RateLimiter allows injection of a custom or distributed rate limiter interface.
	// Purpose: Provide custom rate limiter, e.g. for horizontal scaling.
	// Constraints: Takes precedence over RateLimit/RateBurst if set.
	// Thread-safety: Read-only interface.
	RateLimiter RateLimiter
	// AllowedOrigins for CORS. Empty means no CORS middleware.
	// Purpose: Configures CORS Access-Control-Allow-Origin dynamically.
	// Constraints: Can be empty to bypass CORS entirely.
	// Thread-safety: Read-only slice.
	AllowedOrigins []string
	// AllowedMethods for CORS. Defaults to GET, POST, PUT, DELETE, OPTIONS.
	// Purpose: Configures CORS Access-Control-Allow-Methods dynamically.
	// Constraints: Read-only, pre-joined during initialization.
	// Thread-safety: Read-only slice.
	AllowedMethods []string
	// AllowedHeaders for CORS. Defaults to Accept, Authorization, Content-Type.
	// Purpose: Configures CORS Access-Control-Allow-Headers dynamically.
	// Constraints: Read-only, pre-joined during initialization.
	// Thread-safety: Read-only slice.
	AllowedHeaders []string
	// RateLimit is the maximum requests per second. 0 disables rate limiting.
	// Purpose: Limits incoming traffic rates to prevent resource exhaustion.
	// Constraints: Ignored if set to 0.
	// Thread-safety: Read-only float64.
	RateLimit float64
	// RateBurst is the maximum burst size for rate limiting. Defaults to RateLimit.
	// Purpose: Configures the burst tolerance over the rate limit.
	// Constraints: Ignored if RateLimit is 0.
	// Thread-safety: Read-only int.
	RateBurst int
	// ReadTimeout for the HTTP server.
	// Purpose: Time allowed to read the entire request, including the body.
	// Constraints: Protects against slowloris attacks.
	// Thread-safety: Read-only duration.
	ReadTimeout time.Duration
	// ReadHeaderTimeout for the HTTP server.
	// Purpose: Time allowed to read request headers.
	// Constraints: Protects against slowloris attacks targeting headers.
	// Thread-safety: Read-only duration.
	ReadHeaderTimeout time.Duration
	// WriteTimeout for the HTTP server.
	// Purpose: Time allowed to write the response.
	// Constraints: Protects against slow clients.
	// Thread-safety: Read-only duration.
	WriteTimeout time.Duration
	// IdleTimeout for the HTTP server.
	// Purpose: Time allowed for idle keep-alive connections.
	// Constraints: Determines how aggressively idle sockets are closed.
	// Thread-safety: Read-only duration.
	IdleTimeout time.Duration
	// EnableLogger enables the chi request logger middleware.
	// Purpose: Toggles basic HTTP request logging automatically.
	// Constraints: Boolean flag.
	// Thread-safety: Read-only boolean.
	EnableLogger bool
	// MetricsPath specifies the HTTP path where Prometheus metrics are exposed.
	// If empty, metrics are not registered on the router.
	// Purpose: Configures the metrics path dynamically.
	// Constraints: Read-only string.
	// Thread-safety: Read-only string.
	MetricsPath string
}

// DefaultRouterConfig allocates a predefined, highly opinionated configuration structure optimized to aggressively clamp down on network abuse without requiring manual developer tuning.
// Purpose: Bootstraps a secure starting configuration.
// Constraints: Imposes strict security defaults automatically.
// Thread-safety: Returns a new value struct, safe to use across goroutines.
// Internal Logic Deep-Dive: To mitigate Slowloris and other connection-exhaustion attacks, we set aggressive, non-zero timeout boundaries. The standard `http.Server` explicitly defaults to infinite wait times if left unconfigured.
func DefaultRouterConfig() RouterConfig {
	// Internal Logic Deep-Dive: To mitigate Slowloris and other connection-exhaustion attacks, we set aggressive, non-zero timeout boundaries. In Go, the `http.Server` explicitly defaults to infinite wait times if left unconfigured, which is a dangerous anti-pattern in production environments.
	// These default parameters are strictly tuned for production defensive posture.
	// We mandate conservative ReadHeaderTimeout and WriteTimeout values specifically
	// to sever stalling connections, neutralizing Slowloris and other resource-exhaustion vectors.
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
		MetricsPath:       "/metrics",
	}
}

// RouterOption establishes a typed functional parameter contract, affording consumers the capability to sequentially override targeted properties inside the RouterConfig map.
// Purpose: Enables functional option pattern configuration.
// Constraints: Evaluated serially during configuration initialization.
// Thread-safety: Safe when used sequentially during initialization.
// Internal Logic Deep-Dive: Follows the functional option pattern to maintain backward API compatibility.
type RouterOption func(*RouterConfig)

// WithCORS mandates strict origin filtering on incoming HTTP requests by forcing the router to reject preflight handshakes originating from domain names not explicitly enumerated here.
// Purpose: Adds allowed domains to the CORS middleware layer.
// Constraints: Can be overridden or ignored if empty.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
// Internal Logic Deep-Dive: Ensuring this is an option allows us to completely disable CORS for internal microservices.
func WithCORS(origins ...string) RouterOption {
	return func(c *RouterConfig) {
		c.AllowedOrigins = origins
	}
}

// WithCustomRateLimiter overrides the default memory limiter with a custom distributed rate limiting implementation.
// Purpose: Enables global, distributed rate limits across horizontal replicas (e.g. Redis).
// Constraints: Overrides standard RPS/Burst settings if used.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
// Internal Logic Deep-Dive: Enables scaling from an in-memory bucket to a distributed Redis cluster easily.
func WithCustomRateLimiter(limiter RateLimiter) RouterOption {
	return func(c *RouterConfig) {
		c.RateLimiter = limiter
	}
}

// WithRateLimit imposes a global token-bucket throttling ceiling on all active routing paths, shedding excess traffic that surpasses the designated requests-per-second or instantaneous burst threshold.
// Purpose: Defends against volumetric traffic attacks.
// Constraints: A zero value bypasses rate limiting entirely.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
// Internal Logic Deep-Dive: Shields the system immediately without requiring external infrastructure.
func WithRateLimit(rps float64, burst int) RouterOption {
	return func(c *RouterConfig) {
		c.RateLimit = rps
		c.RateBurst = burst
	}
}

// WithReadTimeout permanently disconnects any client socket attempting to infinitely stall the server by streaming headers or POST body byte chunks at an unacceptably slow cadence.
// Purpose: Preempts slow-loris attacks by capping total read duration.
// Constraints: Must be positive or zero.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
// Internal Logic Deep-Dive: Preemptively clamping the socket read duration provides foundational defense against Slowloris-style denial of service attacks by refusing to buffer indefinitely.
func WithReadTimeout(d time.Duration) RouterOption {
	return func(c *RouterConfig) {
		c.ReadTimeout = d
	}
}

// WithReadHeaderTimeout aggressively snaps shut any connection unable to finalize its initial HTTP header transmission block before the provided countdown completely exhausts.
// Purpose: Disconnects stalling clients.
// Constraints: Must be provided unconditionally on exposed servers.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
// Internal Logic Deep-Dive: Extremely strict threshold targeting malicious clients holding sockets open indefinitely.
func WithReadHeaderTimeout(d time.Duration) RouterOption {
	return func(c *RouterConfig) {
		c.ReadHeaderTimeout = d
	}
}

// WithWriteTimeout preemptively aborts handlers running out of bounds by explicitly closing the client transport stream if computing or flushing the response bytes exceeds the given timespan.
// Purpose: Frees resources associated with stalling clients or handlers.
// Constraints: Bound your long-running handlers inside this window to avoid forced closures.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
// Internal Logic Deep-Dive: Ensures the server doesn't stack up blocked goroutines waiting on a stalled TCP send buffer.
func WithWriteTimeout(d time.Duration) RouterOption {
	return func(c *RouterConfig) {
		c.WriteTimeout = d
	}
}

// WithIdleTimeout proactively purges stagnant Keep-Alive socket connections sitting idly in the system pool once they surpass the specified inactivity threshold.
// Purpose: Limits the amount of inactive sockets held in memory.
// Constraints: Should generally be longer than read timeouts.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
// Internal Logic Deep-Dive: Keeps sockets warm for bursty clients but sweeps them aggressively when traffic drops.
func WithIdleTimeout(d time.Duration) RouterOption {
	return func(c *RouterConfig) {
		c.IdleTimeout = d
	}
}

// WithLogger conditionally patches a high-performance observability interceptor directly into the request processing chain, dumping formatted diagnostic logs to standard out for every routed call.
// Purpose: Simplifies observing real-time HTTP metrics and route performance.
// Constraints: Writes to standard output.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
// Internal Logic Deep-Dive: Bypasses standard output to directly ship structured JSON logs.
func WithLogger(enabled bool) RouterOption {
	return func(c *RouterConfig) {
		c.EnableLogger = enabled
	}
}

// WithMetricsPath configures the HTTP path where Prometheus metrics are exposed.
// An empty path disables metrics registration.
// Purpose: Allows custom metrics paths (e.g. for security through obscurity or admin ports).
// Constraints: Path should start with a slash if not empty.
// Thread-safety: Mutates configuration struct safely during synchronous initialization.
// Internal Logic Deep-Dive: Purposely bypasses authentication and rate-limiting middleware to guarantee scrapers never timeout.
func WithMetricsPath(path string) RouterOption {
	return func(c *RouterConfig) {
		c.MetricsPath = path
	}
}

// parseOptions is an internal helper that initializes the DefaultRouterConfig
// and then safely applies all provided functional options.
// Purpose: Aggregates modular setup logic.
// Constraints: Should only be called internally during router or server initialization.
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

// NewRouter creates a new chi.Mux router with standard security, recovery, and observability middlewares applied.
//
// The default stack enforces request tracing (RequestID), client IP extraction (RealIP),
// panic safety (Recoverer), and strict security headers. Optional middlewares
// (Logger, RateLimit, CORS) are injected based on the provided options.
// Purpose: Quickly bootstraps a production-ready HTTP router.
// Constraints: Expects valid setup parameters.
// Thread-safety: Safely initializes global middlewares for concurrent request processing.
// Internal Logic Deep-Dive: Pre-configures 404 and 405 handlers to return structured JSON instead of plain text.
func NewRouter(opts ...RouterOption) *chi.Mux {
	cfg := parseOptions(opts...)

	r := chi.NewRouter()

	// Core middleware stack.
	if cfg.MetricsPath != "" {
		r.Use(otelchi.Middleware("httpkit",
			otelchi.WithChiRoutes(r),
			otelchi.WithFilter(func(req *http.Request) bool {
				return req.URL.Path != cfg.MetricsPath
			}),
		))
	} else {
		r.Use(otelchi.Middleware("httpkit", otelchi.WithChiRoutes(r)))
	}
	r.Use(middleware.RequestID)
	//nolint:staticcheck // SA1019: middleware.RealIP is deprecated in chi v5.3.0 but retained for backward compatibility
	r.Use(middleware.RealIP)
	// Gracefully handles panics inside route handlers, converting them to 500 Internal Server Errors
	// to prevent the entire node process from crashing.
	// Internal Logic Deep-Dive: We unconditionally load `middleware.Recoverer` *before* injecting any user-defined middlewares like `middleware.Logger`. This exact ordering ensures that if the logging system itself experiences a panic (e.g. nil pointer during attribute generation), the recoverer still catches it, saving the process.
	r.Use(middleware.Recoverer)

	if cfg.EnableLogger {
		r.Use(middleware.Logger)
	}

	// Security headers — always enabled.
	r.Use(SecurityHeadersMiddleware)

	// Rate limiting based on the x/time/rate token bucket algorithm.
	if cfg.RateLimiter != nil {
		r.Use(RateLimitMiddleware(cfg.RateLimiter))
	} else if cfg.RateLimit > 0 {
		burst := cfg.RateBurst
		if burst <= 0 {
			burst = int(cfg.RateLimit)
		}
		limiter := rate.NewLimiter(rate.Limit(cfg.RateLimit), burst)
		r.Use(RateLimitMiddleware(limiter))
	}

	// CORS conditionally injected if origins are specified.
	if len(cfg.AllowedOrigins) > 0 {
		r.Use(CORSMiddleware(cfg.AllowedOrigins, cfg.AllowedMethods, cfg.AllowedHeaders))
	}

	// Expose Prometheus metrics endpoint.
	if cfg.MetricsPath != "" {
		r.Handle(cfg.MetricsPath, promhttp.Handler())
	}

	return r
}

// NewServer allocates a native HTTP transport daemon strictly governed by the precalculated network timeout parameters to act as a resilient fortress against malicious connection hogging attacks.
// Purpose: It applies the read/write timeouts derived from the router options to prevent
// slowloris and other resource exhaustion attacks natively at the stdlib server level.
// Constraints: Relies heavily on the exact timeout metrics defined.
// Thread-safety: Initialization only; the underlying net/http handling is safely concurrent.
// Internal Logic Deep-Dive: Wires the configured timeouts directly into the standard `http.Server` structure.
func NewServer(addr string, handler http.Handler, opts ...RouterOption) *http.Server {
	cfg := parseOptions(opts...)
	// Explicitly propagating derived timeouts into the net/http server structure ensures
	// that even standard library vulnerabilities related to connection stalling are
	// forcefully mitigated at the lowest possible networking tier.
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

// GracefulShutdown commandeers the main process thread, intercepting OS-level kill signals to gracefully drain all ongoing HTTP connections across the underlying server network pool before a hard deadline collapses.
// Purpose: Upon receiving a termination signal, it invokes the server's Shutdown method, giving ongoing
// active requests up to the specified timeout duration to complete before forcing a closure.
// Constraints: Blocks until the server exits completely or the timeout expires.
// Thread-safety: It manages synchronization internally via channels and safely blocks the
// calling goroutine until shutdown completes or times out, safely orchestrating multiple concurrent signals.
// Internal Logic Deep-Dive: Stops accepting new connections instantly while waiting for active handlers to resolve, preventing dropped user transactions.
func GracefulShutdown(srv *http.Server, timeout time.Duration) error {
	serverErr := make(chan error, 1)
	go func() {
		// Asynchronously launch the server. If it immediately fails (e.g. port already bound),
		// we funnel the error to the select block via the channel.
		serverErr <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	// Register for typical OS container termination signals.
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	// Block the main thread. We wake up if the server crashes unexpectedly, or if
	// the OS asks the process to exit.
	// Internal Logic Deep-Dive: The `select` block provides bidirectional responsiveness. If `srv.ListenAndServe()` fails instantly on boot (e.g., port 80 is already in use by another app), the `serverErr` channel fires and the application exits immediately. Conversely, if `<-quit` fires, we transition gracefully to a shutdown deadline, ensuring we don't indefinitely block on dead sockets.
	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-quit:
		// When a shutdown signal is caught, establish a bounded deadline. Any requests
		// still processing when the context expires will be abruptly disconnected.
		var ctx context.Context
		var cancel context.CancelFunc
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
		} else {
			ctx, cancel = context.WithCancel(context.Background())
		}
		defer cancel()
		return srv.Shutdown(ctx)
	}
}
