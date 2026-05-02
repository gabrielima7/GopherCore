// Package httpkit provides HTTP utilities, routing, middleware, and standard responses.
package httpkit

import (
	"net/http"

	"github.com/gabrielima7/GopherCore/jsonutil"
)

// ErrorResponse defines the standard, predictable JSON structure returned to
// clients whenever an API error occurs. This ensures consistent error handling
// on the consumer side. Structurally safe for JSON marshalling.
//
// Purpose: Defines standard layout for JSON API errors.
// Constraints: Assumes error message text is safely sanitized for external viewing.
// Thread-safety: Data structure, safe when not mutated concurrently.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// JSON securely marshals the provided data interface into JSON and writes it
// to the HTTP response with the specified status code. If marshaling fails,
// it returns a generic 500 response without leaking internal structures.
//
// Purpose: Simplifies sending structured JSON to clients securely.
// Constraints: The data payload must be serializable to JSON.
// Thread-safety: Safe for concurrent use across multiple HTTP request handlers.
func JSON(w http.ResponseWriter, status int, data any) {
	body, err := jsonutil.Marshal(data)
	if err != nil {
		http.Error(w, `{"error":"internal server error","code":500}`, http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h["Content-Type"] = []string{"application/json; charset=utf-8"}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// Error constructs and writes an ErrorResponse payload to the client with
// the given HTTP status code and custom error message.
// Purpose: Standardizes JSON error messages.
// Constraints: Status should be a valid HTTP status code.
// Thread-safety: Safe for concurrent use across multiple HTTP request handlers.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Code:    status,
		Message: message,
	})
}

// Ok is a convenience wrapper around JSON that returns an HTTP 200 (OK) status.
// Purpose: Shorthand for returning successful 200 JSON responses.
// Constraints: Relies on json.Marshal internally, meaning data must be marshallable.
// Thread-safety: Safe for concurrent use.
func Ok(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, data)
}

// Created is a convenience wrapper around JSON that returns an HTTP 201 (Created)
// status, typically used after successfully creating a new resource.
// Purpose: Shorthand for returning successful 201 JSON responses.
// Constraints: Relies on json.Marshal internally, meaning data must be marshallable.
// Thread-safety: Safe for concurrent use.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}

// NoContent responds to the client with an HTTP 204 (No Content) status code,
// signaling that the request was successful but there is no body to return
// (e.g., after a successful DELETE operation).
// Purpose: Shorthand for returning successful 204 empty responses.
// Constraints: Does not accept a data payload.
// Thread-safety: Safe for concurrent use.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
