package httpkit

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestJSONResponse_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		data         any
		wantCode     int
		wantBody     string
		wantHeader   string
	}{
		{
			name:       "valid map",
			status:     http.StatusOK,
			data:       map[string]string{"hello": "world"},
			wantCode:   http.StatusOK,
			wantBody:   `{"hello":"world"}`,
			wantHeader: "application/json; charset=utf-8",
		},
		{
			name:       "empty map",
			status:     http.StatusAccepted,
			data:       map[string]string{},
			wantCode:   http.StatusAccepted,
			wantBody:   `{}`,
			wantHeader: "application/json; charset=utf-8",
		},
		{
			name:       "nil interface",
			status:     http.StatusOK,
			data:       nil,
			wantCode:   http.StatusOK,
			wantBody:   `null`,
			wantHeader: "application/json; charset=utf-8",
		},
		{
			name:       "unmarshalable channel",
			status:     http.StatusOK,
			data:       make(chan int),
			wantCode:   http.StatusInternalServerError,
			wantBody:   "{\"error\":\"internal server error\",\"code\":500}\n",
			wantHeader: "text/plain; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			JSON(rr, tt.status, tt.data)

			if rr.Code != tt.wantCode {
				t.Errorf("expected code %d, got %d", tt.wantCode, rr.Code)
			}

			gotBody := rr.Body.String()
			if gotBody != tt.wantBody {
				t.Errorf("expected body %q, got %q", tt.wantBody, gotBody)
			}

			gotHeader := rr.Header().Get("Content-Type")
			if gotHeader != tt.wantHeader {
				t.Errorf("expected Content-Type %q, got %q", tt.wantHeader, gotHeader)
			}
		})
	}
}

func TestErrorResponse_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		message string
	}{
		{
			name:    "not found",
			status:  http.StatusNotFound,
			message: "resource not found",
		},
		{
			name:    "internal server error",
			status:  http.StatusInternalServerError,
			message: "something went wrong",
		},
		{
			name:    "bad request with empty message",
			status:  http.StatusBadRequest,
			message: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			Error(rr, tt.status, tt.message)

			if rr.Code != tt.status {
				t.Errorf("expected code %d, got %d", tt.status, rr.Code)
			}

			var resp ErrorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Code != tt.status {
				t.Errorf("expected JSON code %d, got %d", tt.status, resp.Code)
			}
			if resp.Message != tt.message {
				t.Errorf("expected JSON message %q, got %q", tt.message, resp.Message)
			}
			if resp.Error != http.StatusText(tt.status) {
				t.Errorf("expected JSON error %q, got %q", http.StatusText(tt.status), resp.Error)
			}
		})
	}
}

func TestShorthandResponses_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(http.ResponseWriter)
		wantCode int
		wantBody string
	}{
		{
			name: "Ok",
			fn: func(w http.ResponseWriter) {
				Ok(w, map[string]int{"count": 42})
			},
			wantCode: http.StatusOK,
			wantBody: `{"count":42}`,
		},
		{
			name: "Created",
			fn: func(w http.ResponseWriter) {
				Created(w, map[string]string{"id": "abc-123"})
			},
			wantCode: http.StatusCreated,
			wantBody: `{"id":"abc-123"}`,
		},
		{
			name: "NoContent",
			fn: func(w http.ResponseWriter) {
				NoContent(w)
			},
			wantCode: http.StatusNoContent,
			wantBody: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tt.fn(rr)

			if rr.Code != tt.wantCode {
				t.Errorf("expected code %d, got %d", tt.wantCode, rr.Code)
			}

			gotBody := rr.Body.String()
			if gotBody != tt.wantBody {
				t.Errorf("expected body %q, got %q", tt.wantBody, gotBody)
			}
		})
	}
}

func TestResponse_Concurrency(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	errCh := make(chan error, numGoroutines*3)

	// Test Ok
	for i := 0; i < numGoroutines; i++ {
		go func(val int) {
			defer wg.Done()
			rr := httptest.NewRecorder()
			Ok(rr, map[string]int{"val": val})
			if rr.Code != http.StatusOK {
				errCh <- errors.New("expected Ok to return 200")
			}
		}(i)
	}

	// Test Error
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			rr := httptest.NewRecorder()
			Error(rr, http.StatusBadRequest, "bad data")
			if rr.Code != http.StatusBadRequest {
				errCh <- errors.New("expected Error to return 400")
			}
		}()
	}

	// Test JSON
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			rr := httptest.NewRecorder()
			JSON(rr, http.StatusAccepted, []string{"a", "b"})
			if rr.Code != http.StatusAccepted {
				errCh <- errors.New("expected JSON to return 202")
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrency error: %v", err)
	}
}
