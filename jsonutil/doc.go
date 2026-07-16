// Package jsonutil provides fast JSON encoding and decoding by wrapping
// github.com/goccy/go-json. All functions are API-compatible with encoding/json.
// Purpose: Centralize rapid JSON serialization while masking library dependencies.
// Constraints: Must mimic standard library signatures exactly.
// Thread-safety: Completely stateless and perfectly safe for concurrent use.
package jsonutil
