package httpkit

import (
	"net/http"

	"github.com/gabrielima7/GopherCore/jsonutil"
)

// ErrorResponse represents a standardized error response body.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// JSON writes a JSON response with the given status code and data.
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

// Error writes a standardized error JSON response.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Code:    status,
		Message: message,
	})
}

// Ok writes a JSON response with HTTP 200 and the given data.
func Ok(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, data)
}

// Created writes a JSON response with HTTP 201 and the given data.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}

// NoContent writes an HTTP 204 response with no body.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
