// Package httpkit provides an HTTP toolkit built on go-chi/chi with
// pre-configured security middleware, rate limiting, CORS control,
// and standardized JSON responses.
package httpkit

import (
	"net/http"
	"strings"

	"golang.org/x/time/rate"
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
		// Assigning directly to the Header map bypasses the default key canonicalization
		// overhead of Set(), reducing CPU allocation micro-overheads per request.
		h := w.Header()
		h["Strict-Transport-Security"] = []string{"max-age=63072000; includeSubDomains; preload"}
		h["X-Content-Type-Options"] = []string{"nosniff"}
		h["X-Frame-Options"] = []string{"DENY"}
		h["Referrer-Policy"] = []string{"strict-origin-when-cross-origin"}
		h["Permissions-Policy"] = []string{"camera=(), microphone=(), geolocation=()"}
		h["X-Xss-Protection"] = []string{"1; mode=block"}
		h["Content-Security-Policy"] = []string{"default-src 'self'"}
		next.ServeHTTP(w, r)
	})
}

// RateLimitMiddleware shields the core application router from denial-of-service volumetric floods by actively discarding connections that surpass the mathematically allocated token bucket allowances.
// Purpose: Protects endpoints from abuse and DoS attacks by throttling traffic.
// Constraints: If a request exceeds the permissible limit, it is immediately aborted,
// and an HTTP 429 (Too Many Requests) response is returned to the client along with a Retry-After header.
// Thread-safety: The internal limiter manages its own mutexes and is inherently safe for
// concurrent execution across thousands of requests.
func RateLimitMiddleware(limiter *rate.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Fast-path token bucket evaluation. By dropping traffic instantaneously and
			// providing a static Retry-After header, we mitigate CPU exhaustion attacks
			// securely without executing downstream routing or serialization logic.
			if !limiter.Allow() {
				h := w.Header()
				h["Retry-After"] = []string{"1"}
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
			origin := r.Header.Get("Origin")

			if origin != "" && (allowAll || originsSet[origin]) {
				// We assign directly to the header map structure rather than using w.Header().Set()
				// to avoid lock overhead, since we know these keys are unique and non-colliding here.
				h := w.Header()
				if allowAll {
					h["Access-Control-Allow-Origin"] = []string{"*"}
					// Note: Setting Allow-Credentials with a wildcard origin is explicitly forbidden
					// by browser security models, so it is strictly omitted here.
				} else {
					h["Access-Control-Allow-Origin"] = []string{origin}
					h["Access-Control-Allow-Credentials"] = []string{"true"}
					h["Vary"] = []string{"Origin"}
				}
				h["Access-Control-Allow-Methods"] = []string{methodsStr}
				h["Access-Control-Allow-Headers"] = []string{headersStr}
				// Cache the preflight response in the browser for 24 hours to reduce OPTIONS traffic.
				h["Access-Control-Max-Age"] = []string{"86400"}
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
