package httpkit

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	expectedHeaders := map[string]string{
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains; preload",
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"X-XSS-Protection":          "1; mode=block",
		"Content-Security-Policy":   "default-src 'self'",
	}

	for header, expected := range expectedHeaders {
		got := rr.Header().Get(header)
		if got != expected {
			t.Errorf("header %s: expected %q, got %q", header, expected, got)
		}
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	// Allow 1 request per second, burst of 1.
	limiter := rate.NewLimiter(1, 1)

	handler := RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should pass.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Second request should be rate limited.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") != "1" {
		t.Fatalf("expected Retry-After header")
	}
}

func TestCORSMiddleware(t *testing.T) {
	handler := CORSMiddleware(
		[]string{"https://example.com"},
		[]string{"GET", "POST"},
		[]string{"Content-Type"},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("allowed origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://example.com")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
			t.Fatal("expected CORS origin header")
		}
	})

	t.Run("disallowed origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://evil.com")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("expected no CORS origin header for disallowed origin")
		}
	})

	t.Run("preflight", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "https://example.com")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected 204 for preflight, got %d", rr.Code)
		}
		if rr.Header().Get("Access-Control-Allow-Methods") != "GET, POST" {
			t.Fatalf("expected methods header, got %q", rr.Header().Get("Access-Control-Allow-Methods"))
		}
	})
}

func TestSecurityHeadersMiddlewareConcurrency(t *testing.T) {
	handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const numGoroutines = 100
	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
				errCh <- errors.New("missing security header")
				return
			}
			errCh <- nil
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent test failed: %v", err)
		}
	}
}

func TestRateLimitMiddlewareConcurrency(t *testing.T) {
	// Allow 100 requests per second, burst of 100
	limiter := rate.NewLimiter(100, 100)

	handler := RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const numGoroutines = 100
	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			// Either OK or TooManyRequests is fine, as long as it doesn't panic
			if rr.Code != http.StatusOK && rr.Code != http.StatusTooManyRequests {
				errCh <- fmt.Errorf("unexpected status code: %d", rr.Code)
				return
			}
			errCh <- nil
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent test failed: %v", err)
		}
	}
}

func TestCORSMiddlewareConcurrency(t *testing.T) {
	handler := CORSMiddleware(
		[]string{"https://example.com"},
		[]string{"GET", "POST"},
		[]string{"Content-Type"},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const numGoroutines = 100
	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if i%2 == 0 {
				req.Header.Set("Origin", "https://example.com")
			} else {
				req.Header.Set("Origin", "https://evil.com")
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			origin := rr.Header().Get("Access-Control-Allow-Origin")
			if i%2 == 0 && origin != "https://example.com" {
				errCh <- fmt.Errorf("expected CORS origin header for allowed origin")
				return
			} else if i%2 != 0 && origin != "" {
				errCh <- fmt.Errorf("expected no CORS origin header for disallowed origin")
				return
			}

			errCh <- nil
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent test failed: %v", err)
		}
	}
}

func TestCORSWildcard(t *testing.T) {
	handler := CORSMiddleware(
		[]string{"*"},
		[]string{"GET"},
		[]string{"Content-Type"},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://anything.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("expected wildcard CORS to allow any origin with *")
	}
	if rr.Header().Get("Access-Control-Allow-Credentials") == "true" {
		t.Fatal("expected wildcard CORS to omit credentials")
	}
}

func TestNoOriginHeader(t *testing.T) {
	handler := CORSMiddleware(
		[]string{"https://example.com"},
		[]string{"GET"},
		[]string{"Content-Type"},
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("expected no CORS headers without Origin")
	}
}
