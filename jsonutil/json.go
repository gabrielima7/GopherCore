// Package jsonutil provides fast JSON encoding and decoding by wrapping
// github.com/goccy/go-json. All functions are API-compatible with encoding/json.
package jsonutil

import (
	"io"

	gojson "github.com/goccy/go-json"
)

// Marshal returns the JSON encoding of v. It is a drop-in, thread-safe replacement
// for encoding/json.Marshal, leveraging goccy/go-json for significantly improved
// encoding performance.
func Marshal(v any) ([]byte, error) {
	return gojson.Marshal(v)
}

// MarshalIndent is like Marshal but applies Indent to format the output.
// It is fully thread-safe and safe for concurrent use across multiple goroutines.
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return gojson.MarshalIndent(v, prefix, indent)
}

// Unmarshal parses the JSON-encoded data and stores the result
// in the value pointed to by v. It uses goccy/go-json for high-performance,
// thread-safe parsing and decoding. The target value v must be a non-nil pointer.
func Unmarshal(data []byte, v any) error {
	return gojson.Unmarshal(data, v)
}

// NewEncoder creates a new JSON encoder that writes to w. Unlike the package-level
// Marshal functions, the returned Encoder is generally NOT safe for concurrent use
// by multiple goroutines without explicit synchronization.
func NewEncoder(w io.Writer) *gojson.Encoder {
	return gojson.NewEncoder(w)
}

// NewDecoder creates a new JSON decoder that reads from r. The returned Decoder
// maintains internal state and is NOT safe for concurrent use across multiple
// goroutines without explicit synchronization.
func NewDecoder(r io.Reader) *gojson.Decoder {
	return gojson.NewDecoder(r)
}

// Valid safely and efficiently reports whether data is a valid JSON encoding,
// without allocating the full structures necessary for a complete Unmarshal.
// It is completely thread-safe.
func Valid(data []byte) bool {
	return gojson.Valid(data)
}
