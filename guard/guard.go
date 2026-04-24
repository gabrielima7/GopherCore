// Package guard provides security guard helpers that wrap the go-playground/validator
// library to offer structured validation and basic input sanitization.
// It is designed to be fully thread-safe for concurrent use across multiple goroutines.
package guard

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
	"github.com/microcosm-cc/bluemonday"
)

// validate is the singleton validator instance.
var validate = validator.New()

// htmlPolicy is the singleton bluemonday strict policy instance.
var htmlPolicy = bluemonday.StrictPolicy()

// ValidationError encapsulates the details of a single struct field validation failure.
// It structurally maps the field name, failed validation tag, rejected value, and a
// generated human-readable message. Safe for concurrent access.
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// Error implements the standard error interface for ValidationError, returning the human-readable
// message specific to this single validation failure. It does not contain any thread-unsafe operations.
func (v ValidationError) Error() string {
	return v.Message
}

// ValidationErrors represents a collection of one or more ValidationError instances.
// It is typically generated resulting from a multi-field struct validation failure.
// As a slice of errors, its methods are read-only and thread-safe.
type ValidationErrors []ValidationError

// Error implements the standard error interface for ValidationErrors, aggregating all underlying
// individual field validation messages into a single semicolon-separated string.
// Safe for concurrent access.
func (ve ValidationErrors) Error() string {
	var msgs []string
	for _, e := range ve {
		msgs = append(msgs, e.Message)
	}
	return strings.Join(msgs, "; ")
}

// Validate inspects a given struct using its reflection-based `validate` tags.
// If the struct violates any tags, it aggregates all failures into a ValidationErrors
// slice which implements the error interface. It returns nil if the struct perfectly
// satisfies all validation constraints.
//
// Constraints: The input `s` MUST be a struct or a pointer to a struct, otherwise it returns an error.
// Thread-safety: It relies on a globally initialized validator instance and is entirely
// thread-safe for concurrent use.
func Validate(s any) error {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return err
	}

	errs := make(ValidationErrors, 0, len(validationErrors))
	for _, fe := range validationErrors {
		errs = append(errs, ValidationError{
			Field:   fe.Field(),
			Tag:     fe.Tag(),
			Value:   fmt.Sprintf("%v", fe.Value()),
			Message: formatValidationError(fe),
		})
	}
	return errs
}

// RegisterValidation registers a custom, user-defined validation function mapped to
// a specific tag name. Once registered, this tag can be used in struct fields
// throughout the application. It returns an error if the tag name is already registered.
//
// Thread-safety: This function modifies the global validator instance and is NOT thread-safe
// to call concurrently with active `Validate` calls. It MUST be invoked strictly
// during application startup initialization to prevent data races.
func RegisterValidation(tag string, fn validator.Func) error {
	return validate.RegisterValidation(tag, fn)
}

// SanitizeString performs primitive input scrubbing by stripping out invisible
// Unicode control characters and aggressively trimming leading/trailing whitespace.
// It creates a new allocated string to prevent modifying the original reference.
// Thread-safety: Pure function, safe for concurrent use.
//
// Security Warning: This is purely a basic data-hygiene mechanism and absolutely
// MUST NOT be relied upon as a primary defense against injection attacks like XSS or SQLi.
// Context-aware escaping at the respective boundaries is still strictly required.
func SanitizeString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !unicode.IsControl(r) || r == '\n' || r == '\r' || r == '\t' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// StripHTML aggressively strips all HTML tags, attributes, and potentially dangerous
// payloads from the input string using the microcosm-cc/bluemonday StrictPolicy.
//
// It is explicitly designed to safely handle untrusted user input and mitigate
// Cross-Site Scripting (XSS) vectors by destroying all markup structure, leaving
// only plain text.
// Thread-safety: It leverages a globally instantiated policy and is fully
// safe for concurrent execution across multiple goroutines.
func StripHTML(s string) string {
	return htmlPolicy.Sanitize(s)
}

// formatValidationError analyzes the specific tag that failed validation
// and maps it to a clear, human-readable error message.
//
// Purpose: Acts as the central translation layer between raw validator errors
// and client-friendly HTTP response messages. It switches on the exact tag name.
// Thread-safety: Pure function, safe for concurrent use.
func formatValidationError(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("field '%s' is required", fe.Field())
	case "email":
		return fmt.Sprintf("field '%s' must be a valid email address", fe.Field())
	case "min":
		return fmt.Sprintf("field '%s' must be at least %s", fe.Field(), fe.Param())
	case "max":
		return fmt.Sprintf("field '%s' must be at most %s", fe.Field(), fe.Param())
	case "gte":
		return fmt.Sprintf("field '%s' must be greater than or equal to %s", fe.Field(), fe.Param())
	case "lte":
		return fmt.Sprintf("field '%s' must be less than or equal to %s", fe.Field(), fe.Param())
	case "url":
		return fmt.Sprintf("field '%s' must be a valid URL", fe.Field())
	case "uuid":
		return fmt.Sprintf("field '%s' must be a valid UUID", fe.Field())
	default:
		return fmt.Sprintf("field '%s' failed validation on tag '%s'", fe.Field(), fe.Tag())
	}
}
