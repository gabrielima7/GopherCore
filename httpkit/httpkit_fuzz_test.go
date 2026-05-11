package httpkit

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func FuzzRouter(f *testing.F) {
	f.Add("GET", "/", []byte(""), "Origin", "https://example.com")
	f.Add("POST", "/api/data", []byte(`{"key":"value"}`), "Content-Type", "application/json")
	f.Add("OPTIONS", "/test", []byte(""), "Origin", "http://localhost")
	f.Add("PUT", "/update/123", []byte("garbage data"), "X-Custom-Header", "malicious_string")

	router := NewRouter(
		WithLogger(true),
		WithRateLimit(100, 200),
		WithCORS("https://example.com", "http://localhost", "*"),
	)

	f.Fuzz(func(t *testing.T, method, target string, body []byte, headerKey, headerVal string) {
		var req *http.Request
		func() {
			defer func() {
				_ = recover()
			}()
			req = httptest.NewRequest(method, target, bytes.NewReader(body))
		}()

		if req == nil {
			req = httptest.NewRequest("GET", "/", bytes.NewReader(body))
			req.Method = method
			req.RequestURI = target
		}

		req.Header.Add(headerKey, headerVal)

		rr := httptest.NewRecorder()

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Router panicked: method=%q target=%q body=%q headerKey=%q headerVal=%q panic=%v", method, target, body, headerKey, headerVal, r)
			}
		}()

		router.ServeHTTP(rr, req)
	})
}
