package httpkit

import (
	"net/http"
	"strings"

	"golang.org/x/time/rate"
)

var (
	headerSTS = []string{"max-age=63072000; includeSubDomains; preload"}
	headerCTO = []string{"nosniff"}
	headerXFO = []string{"DENY"}
	headerRP  = []string{"strict-origin-when-cross-origin"}
	headerPP  = []string{"camera=(), microphone=(), geolocation=()"}
	headerXXP = []string{"1; mode=block"}
	headerCSP = []string{"default-src 'self'"}
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
		h["Strict-Transport-Security"] = headerSTS
		h["X-Content-Type-Options"] = headerCTO
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

	methodsSlice := []string{strings.Join(allowedMethods, ", ")}
	headersSlice := []string{strings.Join(allowedHeaders, ", ")}
	credSlice := []string{"true"}
	maxAgeSlice := []string{"86400"}
	varySlice := []string{"Origin"}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" && (allowAll || originsSet[origin]) {
				h := w.Header()
				h["Access-Control-Allow-Origin"] = []string{origin}
				h["Access-Control-Allow-Methods"] = methodsSlice
				h["Access-Control-Allow-Headers"] = headersSlice
				h["Access-Control-Allow-Credentials"] = credSlice
				h["Access-Control-Max-Age"] = maxAgeSlice
				h["Vary"] = varySlice
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
