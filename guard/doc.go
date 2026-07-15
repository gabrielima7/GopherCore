// Package guard provides security guard helpers that wrap the go-playground/validator
// library to offer structured validation and basic input sanitization.
// It is designed to be fully thread-safe for concurrent use across multiple goroutines.
// Purpose: Implements unified input sanitization and struct validation routines.
// Constraints: Relies on the go-playground/validator library for the heavy lifting.
// Thread-safety: Pure and concurrent-safe string operations, globally synchronized validator instance.
package guard
