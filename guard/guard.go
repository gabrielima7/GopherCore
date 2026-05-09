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
// Purpose: Holds the global go-playground/validator instance.
// Constraints: Initialized once at startup.
// Thread-safety: Methods provided by the validator are inherently thread-safe.
var validate = validator.New()

// htmlPolicy is the singleton bluemonday strict policy instance.
// Purpose: Holds the global HTML sanitizer policy instance.
// Constraints: Initialized once at startup.
// Thread-safety: Methods provided by the policy are inherently thread-safe.
var htmlPolicy = bluemonday.StrictPolicy()

// ValidationError represents a discrete, structural failure during struct evaluation, mapping exactly which field violated its bound constraints.
// Purpose: Provides structured field-level error mapping.
// Constraints: Normally generated internally via reflection checks.
// Thread-safety: Safe for concurrent access when not mutated.
// It structurally maps the field name, failed validation tag, rejected value, and a
// generated human-readable message.
type ValidationError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// Error fulfills the standard Go error interface for ValidationError, yielding a pre-formatted, user-safe description of the exact constraint violation.
// Purpose: Allows treating a ValidationError strictly as an error interface.
// Constraints: Standard interface constraint.
// Thread-safety: Pure method on value receiver.
func (v ValidationError) Error() string {
	return v.Message
}

// ValidationErrors aggregates a slice of individual ValidationError items, typically accumulated when inspecting complex struct trees with multiple faulty fields.
// Purpose: Groups multiple validation errors into a single structured response.
// Constraints: Must be iterated over to inspect individual field errors.
// Thread-safety: As a slice of errors, its methods are read-only and thread-safe.
type ValidationErrors []ValidationError

// Error flattens the aggregated ValidationErrors slice into a single, cohesive semicolon-separated string suitable for simple logging sinks.
// Purpose: Flattens grouped errors into a single error interface.
// Constraints: Aggregated string may be large if many fields failed.
// Thread-safety: Safe for concurrent access.
func (ve ValidationErrors) Error() string {
	var msgs []string
	for _, e := range ve {
		msgs = append(msgs, e.Message)
	}
	return strings.Join(msgs, "; ")
}

// Validate rigorously inspects the provided struct (or struct pointer) using reflection to ensure all fields perfectly satisfy their declared `validate` struct tags.
// Purpose: Enforces struct field rules dynamically based on struct tags.
// Constraints: The input `s` MUST be a struct or a pointer to a struct, otherwise it returns an error.
// Thread-safety: It relies on a globally initialized validator instance and is entirely
// thread-safe for concurrent use.
func Validate(s any) error {
	err := validate.Struct(s)
	// Fast path: if the payload fully complies, avoid further reflection or allocations.
	if err == nil {
		return nil
	}

	// Determine if the failure originates from struct rules, or if it's an incompatible type
	// (e.g., passing an integer instead of a struct).
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return err
	}

	// Pre-allocate the exact slice capacity since the number of failed constraints is strictly known.
	// This minimizes heap allocations and reduces GC pressure on heavy validation requests.
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

// RegisterValidation expands the internal ruleset by binding a custom validation function to a new struct tag identifier, permitting application-specific invariant checks.
// Purpose: Extends the validation engine with custom application-specific rules.
// Constraints: MUST be invoked purely during initialization phases.
// Thread-safety: This function modifies the global validator instance and is NOT thread-safe
// to call concurrently with active `Validate` calls. It MUST be invoked strictly
// during application startup initialization to prevent data races.
func RegisterValidation(tag string, fn validator.Func) error {
	return validate.RegisterValidation(tag, fn)
}

// SanitizeString scrubs untrusted user input by systematically destroying invisible Unicode control characters and aggressively stripping surrounding whitespace.
// Purpose: Strips out unwanted whitespace and control characters from strings.
// Constraints: This is purely a basic data-hygiene mechanism and absolutely
// MUST NOT be relied upon as a primary defense against injection attacks like XSS or SQLi.
// Thread-safety: Pure function, safe for concurrent use.
//
// Security Warning: Context-aware escaping at the respective boundaries is still strictly required.
func SanitizeString(s string) string {
	var b strings.Builder
	// Preallocate buffer to the exact length of the input string to avoid reallocation
	// penalties during iterative rune writing. This assumes the output will be close
	// to the original size, optimizing for the happy path.
	b.Grow(len(s))
	for _, r := range s {
		// Allow specific control characters that denote legitimate whitespace formatting
		// (newlines, carriage returns, tabs) while filtering out invisible malicious payloads.
		if !unicode.IsControl(r) || r == '\n' || r == '\r' || r == '\t' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// StripHTML neutralizes markup injection vectors by aggressively eliminating all HTML tags, attributes, and JavaScript payloads from the given string.
// Purpose: Mitigate Cross-Site Scripting (XSS) vectors by destroying all markup structure, leaving only plain text.
// Constraints: Destroys markup, not meant for HTML manipulation where structure should be retained.
// Thread-safety: It leverages a globally instantiated policy and is fully
// safe for concurrent execution across multiple goroutines.
func StripHTML(s string) string {
	return htmlPolicy.Sanitize(s)
}

// formatValidationError analyzes the specific tag that failed validation
// and maps it to a clear, human-readable error message.
// Purpose: Acts as the central translation layer between raw validator errors
// and client-friendly HTTP response messages. It switches on the exact tag name.
// Constraints: Should only receive validation errors thrown from the validator package.
// Thread-safety: Pure function, safe for concurrent use.
func formatValidationError(fe validator.FieldError) string {
	// Evaluates the raw underlying validation tag string and translates it directly
	// into an end-user readable formatting string, protecting downstream clients from
	// needing to parse raw validator syntax constants.
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
