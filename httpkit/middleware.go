package httpkit

import (
	"net/http"
	"strings"

	"golang.org/x/time/rate"
)

// SecurityHeadersMiddleware injects a baseline set of strict HTTP security headers
// into every outbound response. It mitigates common web vulnerabilities like
// MIME-sniffing, clickjacking, and XSS. Safe for concurrent use across requests.
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
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// RateLimitMiddleware enforces global inbound request rate limiting using a token
// bucket algorithm (golang.org/x/time/rate). If a request exceeds the permissible
// limit, it is immediately aborted, and an HTTP 429 (Too Many Requests) response
// is returned to the client along with a Retry-After header. The internal limiter
// manages its own mutexes and is safe for concurrent requests.
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
// It verifies the Origin header against a pre-configured whitelist. It automatically
// intercepts and responds to HTTP OPTIONS preflight requests without passing them down
// the middleware chain. Configuration maps and slices are built during initialization
// and read concurrently during requests, ensuring thread safety.
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
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", methodsStr)
				w.Header().Set("Access-Control-Allow-Headers", headersStr)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.Header().Set("Vary", "Origin")
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
