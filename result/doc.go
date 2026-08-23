// Package result provides a generic Result[T] type that encapsulates
// either a successful value or an error. It follows Go's native error
// handling philosophy — no panics, no exceptions.
// Purpose: Enforce strict monadic error handling across application boundaries.
// Constraints: T can be any type; immutable after creation.
// Thread-safety: Generic result instances are fully thread-safe post-instantiation.
// Internal Logic Deep-Dive: By encapsulating success and error states into a single struct without interface wrapping, we force the compiler to allocate results directly on the stack rather than escaping to the heap.
package result
