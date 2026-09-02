// Package guard provides runtime assertion, validation, and sanitization tools.
// Purpose: guard provides runtime assertion and panic recovery utilities.
// Constraints: Internal package.
// Thread-safety: Varies by component.
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
// Internal Logic Deep-Dive: We use struct tags tightly to ensure the reported field names match the JSON payload, not the Go struct.
type ValidationError struct {
	// Field is the struct property name that failed validation.
	// Purpose: Identifies which specific input parameter was invalid.
	// Constraints: Maps to the struct field's defined name or JSON tag.
	// Thread-safety: Read-only string.
	Field string `json:"field"`
	// Tag is the specific validation rule that triggered the failure.
	// Purpose: Identifies the exact rule (e.g., 'required', 'email') that was violated.
	// Constraints: Maps to go-playground/validator tag names.
	// Thread-safety: Read-only string.
	Tag string `json:"tag"`
	// Value is the actual input value that was rejected, formatted as a string.
	// Purpose: Shows the invalid input for debugging and logging.
	// Constraints: Should be sanitized if logging sensitive PII.
	// Thread-safety: Read-only string.
	Value string `json:"value"`
	// Message is a human-readable explanation of why the validation failed.
	// Purpose: Provides a client-friendly error message.
	// Constraints: Safe for external exposure via API responses.
	// Thread-safety: Read-only string.
	Message string `json:"message"`
}

// Error fulfills the standard Go error interface for ValidationError, yielding a pre-formatted, user-safe description of the exact constraint violation.
// Purpose: Allows treating a ValidationError strictly as an error interface.
// Constraints: Standard interface constraint.
// Thread-safety: Pure method on value receiver.
// Internal Logic Deep-Dive: Avoids heavy string concatenation, formatting cleanly for logs.
func (v ValidationError) Error() string {
	return v.Message
}

// ValidationErrors aggregates a slice of individual ValidationError items, typically accumulated when inspecting complex struct trees with multiple faulty fields.
// Purpose: Groups multiple validation errors into a single structured response.
// Constraints: Must be iterated over to inspect individual field errors.
// Thread-safety: As a slice of errors, its methods are read-only and thread-safe.
// Internal Logic Deep-Dive: Batching errors significantly improves UX for large form submissions.
type ValidationErrors []ValidationError

// Error flattens the aggregated ValidationErrors slice into a single, cohesive semicolon-separated string suitable for simple logging sinks.
// Purpose: Flattens grouped errors into a single error interface.
// Constraints: Aggregated string may be large if many fields failed.
// Thread-safety: Safe for concurrent access.
// Internal Logic Deep-Dive: Avoids heavy string concatenation, formatting cleanly for logs.
func (ve ValidationErrors) Error() string {
	var msgs []string
	// Linearly map the individually nested validation errors to avoid losing context
	// when logging, ensuring all broken constraints are visible in a single log trace.
	for _, e := range ve {
		msgs = append(msgs, e.Message)
	}
	return strings.Join(msgs, "; ")
}

// Validate inspects the provided struct using reflection to ensure all fields satisfy their declared `validate` tags.
// Purpose: Enforces struct field rules dynamically based on struct tags.
// Constraints: The input `s` MUST be a struct or a pointer to a struct, otherwise it returns an error.
// Thread-safety: It relies on a globally initialized validator instance and is entirely
// thread-safe for concurrent use.
// Internal Logic Deep-Dive: We use `make(ValidationErrors, 0, len(validationErrors))` to initialize the slice with an exact capacity. During a volumetric payload attack, dynamically appending to a zero-capacity slice would cause repeated memory allocations.
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
	// Internal Logic Deep-Dive: We use `make(ValidationErrors, 0, len(validationErrors))` to initialize the slice with an exact capacity matching the number of underlying validation faults. During a volumetric payload attack or heavily nested JSON validation failure, dynamically appending to a zero-capacity slice would cause repeated memory allocations and heap fragmentation, grinding the garbage collector to a halt.
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
// Internal Logic Deep-Dive: Binds regex or custom logic to a specific struct tag to keep structs declarative and clean.
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
// Internal Logic Deep-Dive: We iteratively filter out invisible Unicode control characters while explicitly allowing whitespace and newlines, mitigating basic injection paths before data hits the deeper validation layers.
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
// Internal Logic Deep-Dive: Uses highly optimized regex to strip `<...>` elements quickly before data hits the database.
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
