// Package result provides a generic Result[T] type that encapsulates
// either a successful value or an error. It follows Go's native error
// handling philosophy — no panics, no exceptions.
// Purpose: Enforce strict monadic error handling across application boundaries.
// Constraints: T can be any type; immutable after creation.
// Thread-safety: Generic result instances are fully thread-safe post-instantiation.
package result
