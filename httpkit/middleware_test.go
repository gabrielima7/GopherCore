package httpkit

import (
	"go.uber.org/goleak"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
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

func TestMiddleware_TableDriven(t *testing.T) {
	defer goleak.VerifyNone(t)
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	t.Run("SecurityHeadersMiddleware", func(t *testing.T) {
		handler := SecurityHeadersMiddleware(dummyHandler)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		expectedHeaders := map[string]string{
			"Strict-Transport-Security": "max-age=63072000; includeSubDomains; preload",
			"X-Content-Type-Options":    "nosniff",
			"X-Frame-Options":           "DENY",
			"Referrer-Policy":           "strict-origin-when-cross-origin",
			"Permissions-Policy":        "camera=(), microphone=(), geolocation=()",
			"X-Xss-Protection":          "1; mode=block",
			"Content-Security-Policy":   "default-src 'self'",
		}
		for k, v := range expectedHeaders {
			if got := rr.Header().Get(k); got != v {
				t.Errorf("expected %s=%s, got %s", k, v, got)
			}
		}
	})

	t.Run("CORSMiddleware", func(t *testing.T) {
		corsHandler := CORSMiddleware(
			[]string{"https://allowed.com", "*"},
			[]string{"GET", "POST"},
			[]string{"X-Test"},
		)(dummyHandler)

		tests := []struct {
			name              string
			method            string
			origin            string
			expectedStatus    int
			expectedAllowOrig string
			expectedAllowCred string
			expectedAllowMeth string
			expectedAllowHead string
		}{
			{
				name:              "Wildcard Origin Allowed",
				method:            http.MethodGet,
				origin:            "https://random.com",
				expectedStatus:    http.StatusOK,
				expectedAllowOrig: "*",
				expectedAllowCred: "", // credentials not allowed for wildcard
				expectedAllowMeth: "GET, POST",
				expectedAllowHead: "X-Test",
			},
			{
				name:              "Preflight OPTIONS",
				method:            http.MethodOptions,
				origin:            "https://allowed.com",
				expectedStatus:    http.StatusNoContent,
				expectedAllowOrig: "*", // because * is in allowed origins
				expectedAllowCred: "",
				expectedAllowMeth: "GET, POST",
				expectedAllowHead: "X-Test",
			},
			{
				name:              "Missing Origin",
				method:            http.MethodGet,
				origin:            "",
				expectedStatus:    http.StatusOK,
				expectedAllowOrig: "",
				expectedAllowCred: "",
				expectedAllowMeth: "",
				expectedAllowHead: "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest(tt.method, "/", nil)
				if tt.origin != "" {
					req.Header.Set("Origin", tt.origin)
				}
				rr := httptest.NewRecorder()
				corsHandler.ServeHTTP(rr, req)

				if rr.Code != tt.expectedStatus {
					t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
				}
				if got := rr.Header().Get("Access-Control-Allow-Origin"); got != tt.expectedAllowOrig {
					t.Errorf("expected Allow-Origin %q, got %q", tt.expectedAllowOrig, got)
				}
				if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != tt.expectedAllowCred {
					t.Errorf("expected Allow-Credentials %q, got %q", tt.expectedAllowCred, got)
				}
				if tt.expectedAllowOrig != "" {
					if got := rr.Header().Get("Access-Control-Allow-Methods"); got != tt.expectedAllowMeth {
						t.Errorf("expected Allow-Methods %q, got %q", tt.expectedAllowMeth, got)
					}
					if got := rr.Header().Get("Access-Control-Allow-Headers"); got != tt.expectedAllowHead {
						t.Errorf("expected Allow-Headers %q, got %q", tt.expectedAllowHead, got)
					}
				}
			})
		}
	})

	t.Run("CORSMiddleware Explicit Strict Origin", func(t *testing.T) {
		strictCorsHandler := CORSMiddleware(
			[]string{"https://strict.com"},
			[]string{"PUT"},
			[]string{"X-Strict"},
		)(dummyHandler)

		req := httptest.NewRequest(http.MethodPut, "/", nil)
		req.Header.Set("Origin", "https://strict.com")
		rr := httptest.NewRecorder()
		strictCorsHandler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "https://strict.com" {
			t.Errorf("expected specific origin")
		}
		if rr.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Errorf("expected credentials true for strict origin")
		}
	})

	t.Run("RateLimitMiddleware", func(t *testing.T) {
		// Create a limiter that allows exactly 1 request immediately, then none.
		limiter := rate.NewLimiter(0.0001, 1) // extremely slow refill, burst 1
		handler := RateLimitMiddleware(limiter)(dummyHandler)

		// First request (consumes burst)
		req1 := httptest.NewRequest(http.MethodGet, "/", nil)
		rr1 := httptest.NewRecorder()
		handler.ServeHTTP(rr1, req1)
		if rr1.Code != http.StatusOK {
			t.Errorf("expected first request OK, got %d", rr1.Code)
		}

		// Second request (throttled)
		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		rr2 := httptest.NewRecorder()
		handler.ServeHTTP(rr2, req2)
		if rr2.Code != http.StatusTooManyRequests {
			t.Errorf("expected second request 429, got %d", rr2.Code)
		}
		if rr2.Header().Get("Retry-After") != "1" {
			t.Errorf("expected Retry-After: 1, got %q", rr2.Header().Get("Retry-After"))
		}

		// Third request to /metrics (bypasses limiter even though token bucket is exhausted)
		req3 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rr3 := httptest.NewRecorder()
		handler.ServeHTTP(rr3, req3)
		if rr3.Code != http.StatusOK {
			t.Errorf("expected /metrics request to be OK (bypassed), got %d", rr3.Code)
		}
	})

	t.Run("RateLimitMiddleware Nil Limiter", func(t *testing.T) {
		handler := RateLimitMiddleware(nil)(dummyHandler)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected OK status for nil limiter, got %d", rr.Code)
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // pre-cancel context

		tests := []struct {
			name       string
			middleware func(http.Handler) http.Handler
		}{
			{
				name:       "SecurityHeadersMiddleware",
				middleware: SecurityHeadersMiddleware,
			},
			{
				name: "RateLimitMiddleware",
				middleware: func(next http.Handler) http.Handler {
					return RateLimitMiddleware(rate.NewLimiter(10, 10))(next)
				},
			},
			{
				name: "CORSMiddleware",
				middleware: func(next http.Handler) http.Handler {
					return CORSMiddleware([]string{"*"}, []string{"GET"}, []string{})(next)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				handler := tt.middleware(dummyHandler)
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req = req.WithContext(ctx)
				rr := httptest.NewRecorder()

				handler.ServeHTTP(rr, req)

				if rr.Code != 499 {
					t.Errorf("expected status 499 for cancelled context, got %d", rr.Code)
				}
			})
		}
	})
}
