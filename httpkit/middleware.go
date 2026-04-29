package httpkit

import (
	"net/http"
	"strings"

	"golang.org/x/time/rate"
)

// SecurityHeadersMiddleware injects a baseline set of strict HTTP security headers
// into every outbound response. It mitigates common web vulnerabilities like
// MIME-sniffing, clickjacking, and XSS.
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

// RateLimitMiddleware enforces global inbound request rate limiting using a token
// bucket algorithm (golang.org/x/time/rate).
//
// Constraints: If a request exceeds the permissible limit, it is immediately aborted,
// and an HTTP 429 (Too Many Requests) response is returned to the client along with a Retry-After header.
// Thread-safety: The internal limiter manages its own mutexes and is inherently safe for
// concurrent execution across thousands of requests.
func RateLimitMiddleware(limiter *rate.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				w.Header().Set("Retry-After", "1")
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware intercepts incoming requests to manage Cross-Origin Resource Sharing (CORS).
// It verifies the Origin header against a pre-configured whitelist.
//
// Purpose: Enables browser-based cross-origin requests securely.
// Constraints: It automatically intercepts and responds to HTTP OPTIONS preflight requests
// without passing them down the middleware chain.
// Thread-safety: Configuration maps and slices are built during initialization closure time
// and strictly read concurrently during requests, guaranteeing absolute thread safety without mutexes.
func CORSMiddleware(allowedOrigins, allowedMethods, allowedHeaders []string) func(http.Handler) http.Handler {
	originsSet := make(map[string]bool, len(allowedOrigins))
	allowAll := false
	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
		}
		originsSet[o] = true
	}

	methodsStr := strings.Join(allowedMethods, ", ")
	headersStr := strings.Join(allowedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" && (allowAll || originsSet[origin]) {
				h := w.Header()
				h["Access-Control-Allow-Origin"] = []string{origin}
				h["Access-Control-Allow-Methods"] = []string{methodsStr}
				h["Access-Control-Allow-Headers"] = []string{headersStr}
				h["Access-Control-Allow-Credentials"] = []string{"true"}
				h["Access-Control-Max-Age"] = []string{"86400"}
				h["Vary"] = []string{"Origin"}
			}

			// Handle preflight.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
