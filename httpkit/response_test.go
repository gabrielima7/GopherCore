package httpkit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	JSON(rr, http.StatusOK, map[string]string{"hello": "world"})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("expected JSON content type, got %q", rr.Header().Get("Content-Type"))
	}
	body := rr.Body.String()
	if body != `{"hello":"world"}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestErrorResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	Error(rr, http.StatusNotFound, "resource not found")

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	body := rr.Body.String()
	if body == "" {
		t.Fatal("expected non-empty body")
	}
}

func TestOkResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	Ok(rr, map[string]int{"count": 42})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCreatedResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	Created(rr, map[string]string{"id": "abc-123"})

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestNoContentResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	NoContent(rr)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if rr.Body.Len() != 0 {
		t.Fatal("expected empty body")
	}
}

func TestJSONMarshalError(t *testing.T) {
	rr := httptest.NewRecorder()
	// Channels cannot be marshaled to JSON.
	JSON(rr, http.StatusOK, make(chan int))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for unmarshalable type, got %d", rr.Code)
	}
}

func TestResponses_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		fn             func(http.ResponseWriter)
		expectedStatus int
		expectedBody   string
		expectedHeader map[string]string
	}{
		{
			name: "JSON nil payload",
			fn: func(w http.ResponseWriter) {
				JSON(w, http.StatusOK, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `null`,
			expectedHeader: map[string]string{"Content-Type": "application/json; charset=utf-8"},
		},
		{
			name: "JSON empty slice",
			fn: func(w http.ResponseWriter) {
				JSON(w, http.StatusOK, []string{})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `[]`,
			expectedHeader: map[string]string{"Content-Type": "application/json; charset=utf-8"},
		},
		{
			name: "JSON unmarshalable data",
			fn: func(w http.ResponseWriter) {
				JSON(w, http.StatusOK, make(chan int))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"internal server error","code":500}`,
			expectedHeader: map[string]string{"Content-Type": "text/plain; charset=utf-8"}, // http.Error default
		},
		{
			name: "Error standard",
			fn: func(w http.ResponseWriter) {
				Error(w, http.StatusBadRequest, "bad request format")
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"Bad Request","code":400,"message":"bad request format"}`,
			expectedHeader: map[string]string{"Content-Type": "application/json; charset=utf-8"},
		},
		{
			name: "Ok string payload",
			fn: func(w http.ResponseWriter) {
				Ok(w, "success")
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"success"`,
			expectedHeader: map[string]string{"Content-Type": "application/json; charset=utf-8"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tt.fn(rr)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Validate expected headers
			for k, v := range tt.expectedHeader {
				if got := rr.Header().Get(k); got != v {
					// Some http.Error outputs include newline so we use Contains or strict match.
					// We'll stick to strict match but recognize http.Error sets headers before WriteHeader.
					t.Errorf("expected header %s=%s, got %s", k, v, got)
				}
			}

			// Validate body (trimming trailing newlines to make exact matching easier for http.Error outputs)
			body := rr.Body.String()
			if len(body) > 0 && body[len(body)-1] == '\n' {
				body = body[:len(body)-1]
			}
			if body != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, body)
			}
		})
	}
}
