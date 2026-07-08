// Package result provides a generic Result[T] type that encapsulates
// either a successful value or an error. It follows Go's native error
// handling philosophy — no panics, no exceptions.
// Purpose: Enforce strict monadic error handling across application boundaries.
// Constraints: T can be any type; immutable after creation.
// Thread-safety: Generic result instances are fully thread-safe post-instantiation.
package result

import "fmt"

// Result represents either a successful value or an error in an immutable container.
// Purpose: It encourages explicit error handling and functional transformations.
// Constraints: Cannot be mutated after instantiation.
// Thread-safety: All methods on Result are strictly safe for concurrent use since
// the type is entirely immutable by design after creation.
type Result[T any] struct {
	value T
	err   error
	ok    bool
}

// Ok creates a successful Result containing the provided value.
// Purpose: Wraps a raw value into a success state.
// Constraints: The internal error state is implicitly nil.
// Thread-safety: Pure functional constructor.
func Ok[T any](value T) Result[T] {
	return Result[T]{value: value, ok: true}
}

// Err bypasses any expectation of a valid payload, forcefully initializing a failed outcome structure that rigidly isolates the generated error value for downstream inspection.
// Purpose: Wraps a raw error into a failure state.
// Constraints: The internal value state is the zero value for type T.
// Thread-safety: Pure functional constructor.
func Err[T any](err error) Result[T] {
	return Result[T]{err: err, ok: false}
}

// Errf bridges standard printf-style string interpolations directly into a failed monadic outcome state without requiring an intermediate instantiation.
// Purpose: Formats an error string inline and wraps it.
// Constraints: It is a convenience wrapper around fmt.Errorf and Err.
// Thread-safety: Pure functional constructor.
func Errf[T any](format string, args ...any) Result[T] {
	return Result[T]{err: fmt.Errorf(format, args...), ok: false}
}

// Of converts a standard (value, error) return pair into a Result type.
// Purpose: Converts a classic (value, err) return tuple into a Result.
// Constraints: If err is non-nil, it returns an Err result. Otherwise, it wraps the value in an Ok result.
// Thread-safety: Pure functional constructor.
func Of[T any](value T, err error) Result[T] {
	// Internal Logic Deep-Dive: By checking the error unconditionally here, we safely absorb panic-free operations into the monadic pipeline flow, effectively converting classic Go dual-return-value idioms into a unified state representation that cleanly isolates execution failures from subsequent functional mapping sequences.
	if err != nil {
		return Err[T](err)
	}
	return Ok(value)
}

// IsOk exposes a fast, read-only assertion indicating if the underlying envelope safely holds a materialized value free from any error contamination.
// Purpose: Quick boolean check for success.
// Constraints: Must map precisely to the struct `ok` state.
// Thread-safety: Read-only check.
func (r Result[T]) IsOk() bool {
	return r.ok
}

// IsErr provides a direct semantic contradiction to IsOk, signaling precisely whether the encapsulated pipeline was compromised by a runtime error.
// Purpose: Quick boolean check for failure.
// Constraints: Inverts IsOk logically.
// Thread-safety: Read-only check.
func (r Result[T]) IsErr() bool {
	return !r.ok
}

// Unwrap extracts the underlying value and error, bridging the Result type back to idiomatic Go error handling.
// Purpose: This allows the Result container to be bridged back into standard, idiomatic
// Go error handling logic (value, err).
// Constraints: Assumes the consumer will handle the returned error appropriately.
// Thread-safety: Read-only mapping.
func (r Result[T]) Unwrap() (T, error) {
	return r.value, r.err
}

// UnwrapOr executes a guaranteed resolution by extracting the legitimate success value if present, or forcibly pivoting to the provided static fallback object if an error was captured.
// Purpose: Retrieve the value while providing a default on failure.
// Constraints: If the Result encapsulates an error, it ignores the error and
// immediately provides the explicitly provided fallback value instead.
// Thread-safety: Read-only mapping.
func (r Result[T]) UnwrapOr(fallback T) T {
	if r.ok {
		return r.value
	}
	return fallback
}

// UnwrapOrElse guarantees safe extraction while deferring complex fallback derivation to an external callback, guaranteeing the heavy logic is strictly evaded if the main pipeline succeeds.
// Purpose: Retrieve the value while computing a default dynamically on failure.
// Constraints: The fallback function will only be executed if the Result is an Err.
// Thread-safety: Read-only mapping, though the safety depends on the provided fn.
func (r Result[T]) UnwrapOrElse(fn func(error) T) T {
	if r.ok {
		return r.value
	}
	return fn(r.err)
}

// Error extracts the encapsulated error, safely isolating and projecting solely the failure message while totally ignoring any potential successful payload.
// Purpose: Specifically extracts just the error, useful for standard error aggregation.
// Constraints: Returns nil if no error is present.
// Thread-safety: Read-only getter.
func (r Result[T]) Error() error {
	return r.err
}

// Map shifts the current valid state of the result container using an external pure transformation function, propagating any pre-existing errors down the chain without triggering the computation.
// Purpose: Allows chaining operations on the happy path.
// Constraints: If the original Result is an Err, the error is propagated unchanged and fn is never executed.
// Thread-safety: Generates a new immutable Result. Safe as long as fn is safe.
func Map[T any, U any](r Result[T], fn func(T) U) Result[U] {
	if r.ok {
		return Ok(fn(r.value))
	}
	return Err[U](r.err)
}

// FlatMap passes a successful value to a function that also returns a Result, flattening the nested outcome.
// Purpose: Allows chaining operations that themselves may return errors.
// Constraints: This enables elegant chaining of multiple operations that might fail.
// If the original Result is an Err, the error is propagated unchanged.
// Thread-safety: Generates a new immutable Result. Safe as long as fn is safe.
func FlatMap[T any, U any](r Result[T], fn func(T) Result[U]) Result[U] {
	if r.ok {
		return fn(r.value)
	}
	return Err[U](r.err)
}

// String conforms the envelope to the foundational Stringer interface, projecting an instantly recognizable text signature defining exactly whether it holds an Ok or an Err structure.
// Purpose: Simplifies log and console printing for result patterns.
// Constraints: Evaluates formatting functions which might incur minor runtime costs.
// Thread-safety: Read-only stringer.
func (r Result[T]) String() string {
	if r.ok {
		return fmt.Sprintf("Ok(%v)", r.value)
	}
	return fmt.Sprintf("Err(%v)", r.err)
}
