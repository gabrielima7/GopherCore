// Package config provides a unified, reflection-based configuration loader.
// It parses environment variables directly into structured Go types and enforces
// validation constraints via the github.com/go-playground/validator/v10 library,
// ensuring that the application fails to start if vital configurations are missing
// or malformed.
// Purpose: Automatically reads and validates configuration from the environment.
// Constraints: Relies heavily on accurate reflection tags (env, envDefault, validate).
// Thread-safety: Internally thread-safe for reading configurations during the application boot phase.
package config
