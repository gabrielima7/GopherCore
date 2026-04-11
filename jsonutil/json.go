// Package jsonutil provides fast JSON encoding and decoding by wrapping
// github.com/goccy/go-json. All functions are API-compatible with encoding/json.
package jsonutil

import (
	"io"

	gojson "github.com/goccy/go-json"
)

// Marshal returns the JSON encoding of v.
func Marshal(v any) ([]byte, error) {
	return gojson.Marshal(v)
}

// MarshalIndent returns the indented JSON encoding of v.
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return gojson.MarshalIndent(v, prefix, indent)
}

// Unmarshal parses the JSON-encoded data and stores the result
// in the value pointed to by v.
func Unmarshal(data []byte, v any) error {
	return gojson.Unmarshal(data, v)
}

// NewEncoder creates a new JSON encoder that writes to w.
func NewEncoder(w io.Writer) *gojson.Encoder {
	return gojson.NewEncoder(w)
}

// NewDecoder creates a new JSON decoder that reads from r.
func NewDecoder(r io.Reader) *gojson.Decoder {
	return gojson.NewDecoder(r)
}

// Valid reports whether data is a valid JSON encoding.
func Valid(data []byte) bool {
	return gojson.Valid(data)
}
