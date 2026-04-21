package httpkit

import (
	"net/http"
	"strings"

	"golang.org/x/time/rate"
)

var (
	headerHSTS = []string{"max-age=63072000; includeSubDomains; preload"}
	headerXCTO = []string{"nosniff"}
	headerXFO  = []string{"DENY"}
	headerRP   = []string{"strict-origin-when-cross-origin"}
	headerPP   = []string{"camera=(), microphone=(), geolocation=()"}
	headerXXP  = []string{"1; mode=block"}
	headerCSP  = []string{"default-src 'self'"}

	headerCORSAllowCreds = []string{"true"}
	headerCORSMaxAge     = []string{"86400"}
	headerCORSVary       = []string{"Origin"}
)

// SecurityHeadersMiddleware sets HTTP security headers on every response.
// Headers set:
//   - Strict-Transport-Security (HSTS)
//   - X-Content-Type-Options
//   - X-Frame-Options
//   - Referrer-Policy
//   - Permissions-Policy
//   - X-XSS-Protection
//   - Content-Security-Policy (basic default)
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h["Strict-Transport-Security"] = headerHSTS
		h["X-Content-Type-Options"] = headerXCTO
		h["X-Frame-Options"] = headerXFO
		h["Referrer-Policy"] = headerRP
		h["Permissions-Policy"] = headerPP
		h["X-Xss-Protection"] = headerXXP
		h["Content-Security-Policy"] = headerCSP
		next.ServeHTTP(w, r)
	})
}

// RateLimitMiddleware enforces rate limiting using golang.org/x/time/rate.
// When the rate limit is exceeded, it responds with HTTP 429 Too Many Requests.
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

// CORSMiddleware handles CORS preflight requests and sets CORS headers.
func CORSMiddleware(allowedOrigins, allowedMethods, allowedHeaders []string) func(http.Handler) http.Handler {
	originsSet := make(map[string]bool, len(allowedOrigins))
	allowAll := false
	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
		}
		originsSet[o] = true
	}

	methodsStr := []string{strings.Join(allowedMethods, ", ")}
	headersStr := []string{strings.Join(allowedHeaders, ", ")}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" && (allowAll || originsSet[origin]) {
				h := w.Header()
				h["Access-Control-Allow-Origin"] = []string{origin}
				h["Access-Control-Allow-Methods"] = methodsStr
				h["Access-Control-Allow-Headers"] = headersStr
				h["Access-Control-Allow-Credentials"] = headerCORSAllowCreds
				h["Access-Control-Max-Age"] = headerCORSMaxAge
				h["Vary"] = headerCORSVary
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
