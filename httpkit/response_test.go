package httpkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestResponses(t *testing.T) {
	tests := []struct {
		name         string
		method       func(w http.ResponseWriter)
		expectedCode int
		expectedBody string
		expectedCT   string
	}{
		{
			name: "JSON Response",
			method: func(w http.ResponseWriter) {
				JSON(w, http.StatusOK, map[string]string{"hello": "world"})
			},
			expectedCode: http.StatusOK,
			expectedBody: `{"hello":"world"}`,
			expectedCT:   "application/json; charset=utf-8",
		},
		{
			name: "Error Response",
			method: func(w http.ResponseWriter) {
				Error(w, http.StatusNotFound, "resource not found")
			},
			expectedCode: http.StatusNotFound,
			expectedBody: `{"error":"Not Found","code":404,"message":"resource not found"}`,
			expectedCT:   "application/json; charset=utf-8",
		},
		{
			name: "Ok Response",
			method: func(w http.ResponseWriter) {
				Ok(w, map[string]int{"count": 42})
			},
			expectedCode: http.StatusOK,
			expectedBody: `{"count":42}`,
			expectedCT:   "application/json; charset=utf-8",
		},
		{
			name: "Created Response",
			method: func(w http.ResponseWriter) {
				Created(w, map[string]string{"id": "abc-123"})
			},
			expectedCode: http.StatusCreated,
			expectedBody: `{"id":"abc-123"}`,
			expectedCT:   "application/json; charset=utf-8",
		},
		{
			name: "NoContent Response",
			method: func(w http.ResponseWriter) {
				NoContent(w)
			},
			expectedCode: http.StatusNoContent,
			expectedBody: "",
			expectedCT:   "",
		},
		{
			name: "JSON Marshal Error",
			method: func(w http.ResponseWriter) {
				JSON(w, http.StatusOK, make(chan int)) // Unmarshalable
			},
			expectedCode: http.StatusInternalServerError,
			expectedBody: "{\"error\":\"internal server error\",\"code\":500}\n",
			expectedCT:   "text/plain; charset=utf-8",
		},
		{
			name: "JSON Null Data",
			method: func(w http.ResponseWriter) {
				JSON(w, http.StatusOK, nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: "null",
			expectedCT:   "application/json; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tt.method(rr)

			if rr.Code != tt.expectedCode {
				t.Fatalf("expected code %d, got %d", tt.expectedCode, rr.Code)
			}

			body := rr.Body.String()
			if body != tt.expectedBody {
				t.Fatalf("expected body %q, got %q", tt.expectedBody, body)
			}

			ct := rr.Header().Get("Content-Type")
			if ct != tt.expectedCT {
				t.Fatalf("expected Content-Type %q, got %q", tt.expectedCT, ct)
			}
		})
	}
}

func TestJSONResponseConcurrent(t *testing.T) {
	// Validate "Safe for concurrent use" claim
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			rr := httptest.NewRecorder()
			data := map[string]int{"idx": idx}
			JSON(rr, http.StatusOK, data)

			if rr.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rr.Code)
			}

			var decoded map[string]int
			if err := json.Unmarshal(rr.Body.Bytes(), &decoded); err != nil {
				t.Errorf("failed to decode response: %v", err)
			}
			if decoded["idx"] != idx {
				t.Errorf("expected %d, got %d", idx, decoded["idx"])
			}
		}(i)
	}
	wg.Wait()
}
