package httpkit

import (
	"net/http"
	"strings"
)

// SecurityHeadersMiddleware forcibly modifies the outgoing HTTP response stream by embedding a hardened matrix of web security directives designed to repel framing, sniffing, and cross-site scripting attacks.
// Purpose: Protects HTTP endpoints natively against common web vulnerabilities.
// Constraints: Must be applied globally or directly on routes providing web content.
// Thread-safety: Safe for concurrent use across requests. It assigns to map directly
// instead of globally pre-allocating slices to prevent header map data races.
//
// Headers set:
//   - Strict-Transport-Security (HSTS): Forces HTTPS.
//   - X-Content-Type-Options: Prevents MIME-sniffing.
//   - X-Frame-Options: Denies framing (Clickjacking protection).
//   - Referrer-Policy: Restricts referrer data leakage.
//   - Permissions-Policy: Disables camera, microphone, and geolocation access.
//   - X-XSS-Protection: Enables legacy XSS filtering.
//   - Content-Security-Policy: Restricts resource loading to 'self'.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Immediately sever the connection if the client has disconnected or timed out.
		if r.Context().Err() != nil {
			w.WriteHeader(499) // 499 Client Closed Request
			return
		}
		// Using Set ensures canonicalization

		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("X-Xss-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// RateLimiter defines the interface for an external or internal rate limiting algorithm.
// Purpose: Allows substitution of the standard rate limiter with distributed alternatives (e.g., Redis).
// Constraints: Implementations must be thread-safe.
// Thread-safety: Implementations are expected to manage concurrent state natively.
type RateLimiter interface {
	// Allow evaluates whether a single request is permitted to proceed under the current rate limit parameters.
	// Purpose: Determines if a request should be processed or rejected due to rate limiting.
	// Constraints: Must execute rapidly (O(1)) without blocking the main router thread.
	// Thread-safety: Implementations must be natively thread-safe.
	Allow() bool
}

// RateLimitMiddleware shields the core application router from denial-of-service volumetric floods by actively discarding connections that surpass the mathematically allocated token bucket allowances.
// Purpose: Protects endpoints from abuse and DoS attacks by throttling traffic.
// Constraints: If a request exceeds the permissible limit, it is immediately aborted,
// and an HTTP 429 (Too Many Requests) response is returned to the client along with a Retry-After header.
// Thread-safety: The internal limiter manages its own mutexes and is inherently safe for
// concurrent execution across thousands of requests.
func RateLimitMiddleware(limiter RateLimiter) func(http.Handler) http.Handler {
	if limiter == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Immediately sever the connection if the client has disconnected or timed out.
			if r.Context().Err() != nil {
				w.WriteHeader(499)
				return
			}
			// Bypass rate limiting for telemetry metrics endpoint to prevent monitoring blackouts.
			if r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}
			// Fast-path token bucket evaluation. By dropping traffic instantaneously and
			// providing a static Retry-After header, we mitigate CPU exhaustion attacks
			// securely without executing downstream routing or serialization logic.
			if !limiter.Allow() {
				w.Header().Set("Retry-After", "1")
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware rigorously negotiates browser preflight constraints, inspecting the network Origin header and dynamically injecting explicit Access-Control allowances exclusively for whitelisted domains.
// Purpose: Enables browser-based cross-origin requests securely.
// Constraints: It automatically intercepts and responds to HTTP OPTIONS preflight requests
// without passing them down the middleware chain.
// Thread-safety: Configuration maps and slices are built during initialization closure time
// and strictly read concurrently during requests, guaranteeing absolute thread safety without mutexes.
func CORSMiddleware(allowedOrigins, allowedMethods, allowedHeaders []string) func(http.Handler) http.Handler {
	// Pre-compute O(1) origin lookup map to keep handler execution extremely fast during heavy loads.
	// Internal Logic Deep-Dive: We compute `originsSet`, `methodsStr`, and `headersStr` exactly once during middleware initialization closure. If we ran `strings.Join` or iterated over the `allowedOrigins` slice dynamically per-request inside the handler, the CPU overhead would spike catastrophically under heavy HTTP traffic.
	originsSet := make(map[string]bool, len(allowedOrigins))
	allowAll := false
	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
		}
		originsSet[o] = true
	}

	// Pre-join string slices to avoid executing strings.Join on every single request.
	methodsStr := strings.Join(allowedMethods, ", ")
	headersStr := strings.Join(allowedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Immediately sever the connection if the client has disconnected or timed out.
			if r.Context().Err() != nil {
				w.WriteHeader(499)
				return
			}
			origin := r.Header.Get("Origin")

			if origin != "" && (allowAll || originsSet[origin]) {
				// We use w.Header().Set() for canonicalization

				if allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
					// Note: Setting Allow-Credentials with a wildcard origin is explicitly forbidden
					// by browser security models, so it is strictly omitted here.
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Allow-Methods", methodsStr)
				w.Header().Set("Access-Control-Allow-Headers", headersStr)
				// Cache the preflight response in the browser for 24 hours to reduce OPTIONS traffic.
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			// Preflight requests (OPTIONS) shouldn't reach the actual API handlers. Break the chain.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
