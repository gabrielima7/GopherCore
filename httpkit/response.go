package httpkit

import (
	"net/http"

	"github.com/gabrielima7/GopherCore/jsonutil"
)

// ErrorResponse defines the standard, predictable JSON structure returned to
// clients whenever an API error occurs. This ensures consistent error handling
// on the consumer side.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// JSON securely marshals the provided data interface into JSON and writes it
// to the HTTP response with the specified status code. If marshaling fails,
// it logs an internal error and returns a generic 500 response. Safe for concurrent use.
func JSON(w http.ResponseWriter, status int, data any) {
	body, err := jsonutil.Marshal(data)
	if err != nil {
		http.Error(w, `{"error":"internal server error","code":500}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// Error constructs and writes an ErrorResponse payload to the client with
// the given HTTP status code and custom error message.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Code:    status,
		Message: message,
	})
}

// Ok is a convenience wrapper around JSON that returns an HTTP 200 (OK) status.
func Ok(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, data)
}

// Created is a convenience wrapper around JSON that returns an HTTP 201 (Created)
// status, typically used after successfully creating a new resource.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}

// NoContent responds to the client with an HTTP 204 (No Content) status code,
// signaling that the request was successful but there is no body to return
// (e.g., after a successful DELETE operation).
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
