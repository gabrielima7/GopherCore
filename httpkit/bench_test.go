package httpkit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func OptimizedSecurityHeadersMiddleware(next http.Handler) http.Handler {
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

func BenchmarkSecurityHeadersMiddleware(b *testing.B) {
	handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr.HeaderMap = make(http.Header)
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkOptimizedSecurityHeadersMiddleware(b *testing.B) {
	handler := OptimizedSecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr.HeaderMap = make(http.Header)
		handler.ServeHTTP(rr, req)
	}
}
