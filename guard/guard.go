// Package guard provides security guard helpers that wrap the go-playground/validator
// library to offer structured validation and basic input sanitization.
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

// ValidationError represents a single field validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (v ValidationError) Error() string {
	return v.Message
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

// Error implements the error interface.
func (ve ValidationErrors) Error() string {
	var msgs []string
	for _, e := range ve {
		msgs = append(msgs, e.Message)
	}
	return strings.Join(msgs, "; ")
}

// Validate validates a struct using its field tags and returns structured
// validation errors. Returns nil if validation passes.
func Validate(s any) error {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return err
	}

	var errs ValidationErrors
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

// RegisterValidation registers a custom validation function.
func RegisterValidation(tag string, fn validator.Func) error {
	return validate.RegisterValidation(tag, fn)
}

// SanitizeString removes control characters and trims whitespace
// from the input string. This is a basic sanitization — not a replacement
// for proper escaping at the boundary (SQL, HTML, etc.).
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

// StripHTML removes HTML tags from the input string using the rigorous
// bluemonday strict policy, which is suitable for untrusted HTML and
// mitigating XSS risks.
func StripHTML(s string) string {
	return htmlPolicy.Sanitize(s)
}

// formatValidationError returns a human-readable error message for a validator.FieldError.
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
