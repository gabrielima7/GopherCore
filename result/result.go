// Package result provides a generic Result[T] type that encapsulates
// either a successful value or an error. It follows Go's native error
// handling philosophy — no panics, no exceptions.
package result

import "fmt"

// Result is a generic container representing the outcome of an operation
// that can either succeed with a value of type T, or fail with an error.
//
// Purpose: It encourages explicit error handling and functional transformations.
// Constraints: Cannot be mutated after instantiation.
// Thread-safety: All methods on Result are strictly safe for concurrent use since
// the type is entirely immutable by design after creation.
type Result[T any] struct {
	value T
	err   error
	ok    bool
}

// Ok creates and returns a successful Result encapsulating the provided value.
//
// Purpose: Wraps a raw value into a success state.
// Constraints: The internal error state is implicitly nil.
// Thread-safety: Pure functional constructor.
func Ok[T any](value T) Result[T] {
	return Result[T]{value: value, ok: true}
}

// Err creates and returns a failed Result encapsulating the provided error.
//
// Purpose: Wraps a raw error into a failure state.
// Constraints: The internal value state is the zero value for type T.
// Thread-safety: Pure functional constructor.
func Err[T any](err error) Result[T] {
	return Result[T]{err: err, ok: false}
}

// Errf constructs and returns a failed Result containing a formatted error message.
//
// Purpose: Formats an error string inline and wraps it.
// Constraints: It is a convenience wrapper around fmt.Errorf and Err.
// Thread-safety: Pure functional constructor.
func Errf[T any](format string, args ...any) Result[T] {
	return Result[T]{err: fmt.Errorf(format, args...), ok: false}
}

// Of builds a Result by seamlessly encapsulating the standard Go tuple
// return pattern (value T, err error).
//
// Purpose: Converts a classic (value, err) return tuple into a Result.
// Constraints: If err is non-nil, it returns an Err result. Otherwise, it wraps the value in an Ok result.
// Thread-safety: Pure functional constructor.
func Of[T any](value T, err error) Result[T] {
	if err != nil {
		return Err[T](err)
	}
	return Ok(value)
}

// IsOk evaluates the internal state and returns true exclusively if the
// Result represents a successful outcome containing a value.
// Purpose: Quick boolean check for success.
// Constraints: Must map precisely to the struct `ok` state.
// Thread-safety: Read-only check.
func (r Result[T]) IsOk() bool {
	return r.ok
}

// IsErr evaluates the internal state and returns true exclusively if the
// Result encapsulates a failure or an error.
// Purpose: Quick boolean check for failure.
// Constraints: Inverts IsOk logically.
// Thread-safety: Read-only check.
func (r Result[T]) IsErr() bool {
	return !r.ok
}

// Unwrap safely extracts and returns both the internal value and the error.
//
// Purpose: This allows the Result container to be bridged back into standard, idiomatic
// Go error handling logic (value, err).
// Constraints: Assumes the consumer will handle the returned error appropriately.
// Thread-safety: Read-only mapping.
func (r Result[T]) Unwrap() (T, error) {
	return r.value, r.err
}

// UnwrapOr safely extracts the value if the Result is successful.
//
// Purpose: Retrieve the value while providing a default on failure.
// Constraints: If the Result encapsulates an error, it ignores the error and
// immediately returns the explicitly provided fallback value instead.
// Thread-safety: Read-only mapping.
func (r Result[T]) UnwrapOr(fallback T) T {
	if r.ok {
		return r.value
	}
	return fallback
}

// UnwrapOrElse acts like UnwrapOr, but instead of taking a static fallback value,
// it invokes the provided function fn with the encapsulated error to lazily compute
// and return a dynamic fallback value.
// Purpose: Retrieve the value while computing a default dynamically on failure.
// Constraints: The fallback function will only be executed if the Result is an Err.
// Thread-safety: Read-only mapping, though the safety depends on the provided fn.
func (r Result[T]) UnwrapOrElse(fn func(error) T) T {
	if r.ok {
		return r.value
	}
	return fn(r.err)
}

// Error implements a safety accessor, returning the encapsulated error if the
// Result represents a failure, or nil if the operation was successful.
// Purpose: Specifically extracts just the error, useful for standard error aggregation.
// Constraints: Returns nil if no error is present.
// Thread-safety: Read-only getter.
func (r Result[T]) Error() error {
	return r.err
}

// Map transforms the underlying value of a successful Result[T] into a Result[U]
// by applying the provided function fn.
//
// Purpose: Allows chaining operations on the happy path.
// Constraints: If the original Result is an Err, the error is propagated unchanged and fn is never executed.
// Thread-safety: Generates a new immutable Result. Safe as long as fn is safe.
func Map[T any, U any](r Result[T], fn func(T) U) Result[U] {
	if r.ok {
		return Ok(fn(r.value))
	}
	return Err[U](r.err)
}

// FlatMap applies a fallible function fn to the underlying value of a successful
// Result[T], returning the resulting Result[U].
//
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

// String implements the fmt.Stringer interface to provide a clear, human-readable
// representation of the Result's internal state (e.g., "Ok(value)" or "Err(error)").
// Purpose: Simplifies log and console printing for result patterns.
// Constraints: Evaluates formatting functions which might incur minor runtime costs.
// Thread-safety: Read-only stringer.
func (r Result[T]) String() string {
	if r.ok {
		return fmt.Sprintf("Ok(%v)", r.value)
	}
	return fmt.Sprintf("Err(%v)", r.err)
}
