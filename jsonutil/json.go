// Package jsonutil provides utilities.
// Purpose: jsonutil provides strict JSON encoding and decoding utilities.
// Constraints: Internal package.
// Thread-safety: Varies by component.
package jsonutil

import (
	"io"

	gojson "github.com/goccy/go-json"
)

// Marshal rapidly traverses an arbitrary Go data structure, directly translating its values into a high-performance, strictly valid JSON byte sequence.
// Purpose: It is a drop-in replacement for encoding/json.Marshal, leveraging goccy/go-json
// for significantly improved encoding performance.
// Constraints: Can fail if standard data structures are not encodable.
// Thread-safety: Completely stateless and safe for concurrent use across multiple goroutines.
// Internal Logic Deep-Dive: This wrapper forces the application to use our centrally designated JSON encoder, preventing fragmentation and allowing us to transparently swap the backend engine for performance gains later.
func Marshal(v any) ([]byte, error) {
	return gojson.Marshal(v)
}

// MarshalIndent mirrors standard encoding logic but intelligently injects syntactical whitespace and line breaks to maximize structural readability for human operators.
// Purpose: Formatting JSON structurally.
// Constraints: Output is significantly larger, should only be used for debugging/logging.
// Thread-safety: It is fully thread-safe and safe for concurrent use across multiple goroutines.
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	// Internal Logic Deep-Dive: goccy/go-json provides significant performance improvements for indented serialization by optimizing buffer allocations when inserting whitespace.
	return gojson.MarshalIndent(v, prefix, indent)
}

// Unmarshal systematically deciphers a raw JSON byte blob, meticulously mapping its internal keys onto the properties of a dynamically provided target Go pointer.
// Purpose: Extensively used to convert untrusted payload buffers into structs.
// Constraints: The target value v must be a non-nil pointer.
// Thread-safety: It uses goccy/go-json for high-performance, inherently thread-safe decoding.
func Unmarshal(data []byte, v any) error {
	return gojson.Unmarshal(data, v)
}

// NewEncoder constructs an active streaming serializer designed to progressively flush transformed JSON chunks immediately down the associated abstract pipe sink.
// Purpose: Allows streaming JSON encoding to an io.Writer.
// Constraints: Directly wraps the underlying io.Writer stream state.
// Thread-safety: Unlike the package-level Marshal functions, the returned Encoder
// is generally NOT safe for concurrent use by multiple goroutines without explicit synchronization.
func NewEncoder(w io.Writer) *gojson.Encoder {
	return gojson.NewEncoder(w)
}

// NewDecoder initializes a continuously running translation pipeline, designed to consume and map incoming buffered segments from an actively streaming external origin.
// Purpose: Allows streaming JSON decoding from an io.Reader.
// Constraints: Interacts dynamically with the incoming io.Reader bytes logic.
// Thread-safety: The returned Decoder maintains internal state and is NOT safe
// for concurrent use across multiple goroutines without explicit synchronization.
func NewDecoder(r io.Reader) *gojson.Decoder {
	return gojson.NewDecoder(r)
}

// Valid conducts an ultra-low overhead syntactical sweep over the provided bytes to categorically verify structural JSON compliance without invoking memory-heavy allocations.
// Purpose: Fast check to verify JSON payload validity.
// Constraints: Executes syntactical validation without allocating the full
// structures necessary for a complete Unmarshal.
// Thread-safety: Pure and completely thread-safe.
func Valid(data []byte) bool {
	// Internal Logic Deep-Dive: We explicitly defer to the underlying goccy engine here. By calling `gojson.Valid(data)` instead of attempting a full Unmarshal into `any`, the runtime completely bypasses reflection allocations and heap escapes, keeping memory profiles flat during malicious high-volume payload attacks.
	// Defers strictly to goccy's validation engine to skip reflection overhead
	// entirely, allowing extremely cheap syntactical payload verification.
	return gojson.Valid(data)
}
