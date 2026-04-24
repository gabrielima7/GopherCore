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
