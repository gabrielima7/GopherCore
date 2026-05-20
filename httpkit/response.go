// Package httpkit provides an HTTP toolkit built on go-chi/chi with
// pre-configured security middleware, rate limiting, CORS control,
// and standardized JSON responses.
package httpkit

import (
	"log/slog"
	"net/http"

	"github.com/gabrielima7/GopherCore/jsonutil"
)

// ErrorResponse standardizes the JSON contract across the entire external-facing API, guaranteeing that all client applications receive identically structured payloads when HTTP errors occur.
// Purpose: Defines standard layout for JSON API errors.
// Constraints: Assumes error message text is safely sanitized for external viewing.
// Thread-safety: Data structure, safe when not mutated concurrently.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// JSON intercepts arbitrary Go structures, dynamically encoding them into network-bound JSON bytes, and automatically flushes the results down the socket alongside a forced Content-Type header.
// Purpose: Simplifies sending structured JSON to clients securely.
// Constraints: The data payload must be serializable to JSON.
// Thread-safety: Safe for concurrent use across multiple HTTP request handlers.
func JSON(w http.ResponseWriter, status int, data any) {
	body, err := jsonutil.Marshal(data)
	if err != nil {
		// If serialization completely fails (e.g. unsupported types like channels),
		// we fallback to a hardcoded standard 500 JSON response to avoid breaking API contracts.
		http.Error(w, `{"error":"internal server error","code":500}`, http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h["Content-Type"] = []string{"application/json; charset=utf-8"}
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		slog.Error("failed to write response body", "error", err)
	}
}

// Error fabricates a meticulously structured ErrorResponse dictionary mapped to the corresponding HTTP status code and instantly broadcasts it to the connecting client socket.
// Purpose: Standardizes JSON error messages.
// Constraints: Status should be a valid HTTP status code.
// Thread-safety: Safe for concurrent use across multiple HTTP request handlers.
func Error(w http.ResponseWriter, status int, message string) {
	// Standardize error structures so downstream API consumers can reliably parse
	// fault states without writing bespoke parsing logic for every different endpoint.
	JSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Code:    status,
		Message: message,
	})
}

// Ok provides a rapid, ergonomic shortcut for transmitting successful HTTP 200 JSON payloads, delegating the heavy lifting of serialization entirely to the underlying JSON emitter.
// Purpose: Shorthand for returning successful 200 JSON responses.
// Constraints: Relies on json.Marshal internally, meaning data must be marshallable.
// Thread-safety: Safe for concurrent use.
func Ok(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, data)
}

// Created dispatches a specialized HTTP 201 status code back to the client, semantically indicating that a requested resource was structurally synthesized and stored on the server.
// Purpose: Shorthand for returning successful 201 JSON responses.
// Constraints: Relies on json.Marshal internally, meaning data must be marshallable.
// Thread-safety: Safe for concurrent use.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}

// NoContent silences the HTTP connection pipe entirely, emitting an empty 204 status header to conclusively signal that an operation like a deletion succeeded without requiring further data exchange.
// Purpose: Shorthand for returning successful 204 empty responses.
// Constraints: Does not accept a data payload.
// Thread-safety: Safe for concurrent use.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
