// Package jsonutil provides fast JSON encoding and decoding by wrapping
// github.com/goccy/go-json. All functions are API-compatible with encoding/json.
// Purpose: Centralize rapid JSON serialization while masking library dependencies.
// Constraints: Must mimic standard library signatures exactly.
// Thread-safety: Completely stateless and perfectly safe for concurrent use.
// Internal Logic Deep-Dive: By standardizing around a high-performance custom JSON library API-compatible with the standard library, we bypass heavy reflection-based allocations during hot-path API serialization.
package jsonutil
