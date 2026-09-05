// Package httpkit provides HTTP routing, middleware, and standard responses.
// Purpose: httpkit provides HTTP routing, middleware, and structured JSON responses.
// Constraints: Internal package.
// Thread-safety: Varies by component.
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
// Internal Logic Deep-Dive: Using a consistent wrapper simplifies frontend state management drastically.
type ErrorResponse struct {
	// Error provides a machine-readable string describing the general failure category.
	// Purpose: Provides a programmatic identifier for the error type.
	// Constraints: Usually tied to the HTTP status text.
	// Thread-safety: Read-only string.
	Error string `json:"error"`
	// Code matches the HTTP status code returned in the response header.
	// Purpose: Mirrors the HTTP status code in the JSON payload for easy client parsing.
	// Constraints: Must be a valid HTTP status code.
	// Thread-safety: Read-only integer.
	Code int `json:"code"`
	// Message offers an optional, human-readable description intended for developers or end-users.
	// Purpose: Provides additional context about the failure.
	// Constraints: Should not contain sensitive system information.
	// Thread-safety: Read-only string.
	Message string `json:"message,omitempty"`
}

// JSON intercepts arbitrary Go structures, dynamically encoding them into network-bound JSON bytes, and automatically flushes the results down the socket alongside a forced Content-Type header.
// Purpose: Simplifies sending structured JSON to clients securely.
// Constraints: The data payload must be serializable to JSON.
// Thread-safety: Safe for concurrent use across multiple HTTP request handlers.
// Internal Logic Deep-Dive: Explicitly forces the `Content-Type: application/json` header to prevent MIME-sniffing vulnerabilities in browsers.
func JSON(w http.ResponseWriter, status int, data any) {
	body, err := jsonutil.Marshal(data)
	if err != nil {
		// Internal Logic Deep-Dive: If serialization completely fails (e.g. passing an unsupported type like a channel into the payload), we fallback to a hardcoded standard 500 JSON string response. We explicitly DO NOT call `Error()` here to construct the response because if the root cause was a fundamental marshalling failure, trying to recursively marshal a generic `ErrorResponse` could trigger an infinite loop or a secondary panic.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error","code":500}`))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		slog.Warn("failed to write response body", "error", err)
	}
}

// Error creates an ErrorResponse mapped to the HTTP status code and writes it as JSON to the client.
// Purpose: Standardizes JSON error messages.
// Constraints: Status should be a valid HTTP status code.
// Thread-safety: Safe for concurrent use across multiple HTTP request handlers.
// Internal Logic Deep-Dive: Wraps the internal failure into the explicit ErrorResponse struct to guarantee schema compliance.
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
// Internal Logic Deep-Dive: Reduces boilerplate significantly across hundreds of handler functions.
func Ok(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, data)
}

// Created sends an HTTP 201 status code with the provided data as a JSON payload, indicating resource creation.
// Purpose: Shorthand for returning successful 201 JSON responses.
// Constraints: Relies on json.Marshal internally, meaning data must be marshallable.
// Thread-safety: Safe for concurrent use.
// Internal Logic Deep-Dive: Indicates to the client that their POST command successfully persisted data.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}

// NoContent silences the HTTP connection pipe entirely, emitting an empty 204 status header to conclusively signal that an operation like a deletion succeeded without requiring further data exchange.
// Purpose: Shorthand for returning successful 204 empty responses.
// Constraints: Does not accept a data payload.
// Thread-safety: Safe for concurrent use.
// Internal Logic Deep-Dive: Highly optimized for DELETE operations, minimizing network transfer.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
