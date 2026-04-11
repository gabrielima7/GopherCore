package httpkit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRouter(t *testing.T) {
	r := NewRouter(
		WithCORS("https://example.com"),
		WithRateLimit(1000, 2000),
		WithLogger(false),
	)
	if r == nil {
		t.Fatal("expected non-nil router")
	}

	// Add a test route.
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		Ok(w, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Security headers should be set.
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("expected security headers")
	}
}

func TestNewRouterDefaultConfig(t *testing.T) {
	r := NewRouter()
	if r == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestNewRouterWithDisabledRateLimit(t *testing.T) {
	r := NewRouter(
		WithRateLimit(0, 0), // Disabled
		WithLogger(false),
	)
	if r == nil {
		t.Fatal("expected non-nil router")
	}
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Multiple requests should all succeed.
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rr.Code)
		}
	}
}

func TestNewRouterRateBurstZeroDefaultsToBurst(t *testing.T) {
	// RateLimit > 0 but RateBurst <= 0 → burst defaults to int(RateLimit).
	r := NewRouter(
		WithRateLimit(10, 0),
		WithLogger(false),
	)
	if r == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestNewServer(t *testing.T) {
	r := NewRouter(WithLogger(false))
	srv := NewServer(":8080", r, WithReadTimeout(5*time.Second), WithWriteTimeout(5*time.Second))
	if srv.Addr != ":8080" {
		t.Fatalf("expected :8080, got %s", srv.Addr)
	}
	if srv.ReadTimeout != 5*time.Second {
		t.Fatalf("expected 5s read timeout, got %v", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 5*time.Second {
		t.Fatalf("expected 5s write timeout, got %v", srv.WriteTimeout)
	}
}

func TestDefaultRouterConfig(t *testing.T) {
	cfg := DefaultRouterConfig()
	if cfg.RateLimit != 100 {
		t.Fatalf("expected 100, got %f", cfg.RateLimit)
	}
	if cfg.RateBurst != 200 {
		t.Fatalf("expected 200, got %d", cfg.RateBurst)
	}
	if cfg.ReadTimeout != 15*time.Second {
		t.Fatalf("expected 15s, got %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 15*time.Second {
		t.Fatalf("expected 15s, got %v", cfg.WriteTimeout)
	}
	if !cfg.EnableLogger {
		t.Fatal("expected logger enabled by default")
	}
}
