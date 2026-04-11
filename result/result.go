// Package result provides a generic Result[T] type that encapsulates
// either a successful value or an error. It follows Go's native error
// handling philosophy — no panics, no exceptions.
package result

import "fmt"

// Result represents the outcome of an operation that can either
// succeed with a value of type T, or fail with an error.
type Result[T any] struct {
	value T
	err   error
	ok    bool
}

// Ok creates a successful Result containing the given value.
func Ok[T any](value T) Result[T] {
	return Result[T]{value: value, ok: true}
}

// Err creates a failed Result containing the given error.
func Err[T any](err error) Result[T] {
	return Result[T]{err: err, ok: false}
}

// Errf creates a failed Result with a formatted error message.
func Errf[T any](format string, args ...any) Result[T] {
	return Result[T]{err: fmt.Errorf(format, args...), ok: false}
}

// Of creates a Result from a value and error pair, which is the
// standard Go return pattern: val, err := someFunc().
func Of[T any](value T, err error) Result[T] {
	if err != nil {
		return Err[T](err)
	}
	return Ok(value)
}

// IsOk returns true if the Result contains a successful value.
func (r Result[T]) IsOk() bool {
	return r.ok
}

// IsErr returns true if the Result contains an error.
func (r Result[T]) IsErr() bool {
	return !r.ok
}

// Unwrap returns the value and error. This follows Go's idiomatic
// (value, error) return pattern.
func (r Result[T]) Unwrap() (T, error) {
	return r.value, r.err
}

// UnwrapOr returns the value if Ok, or the provided fallback if Err.
func (r Result[T]) UnwrapOr(fallback T) T {
	if r.ok {
		return r.value
	}
	return fallback
}

// UnwrapOrElse returns the value if Ok, or calls fn to produce a fallback.
func (r Result[T]) UnwrapOrElse(fn func(error) T) T {
	if r.ok {
		return r.value
	}
	return fn(r.err)
}

// Error returns the error if present, or nil if Ok.
func (r Result[T]) Error() error {
	return r.err
}

// Map transforms the value inside a Result using fn, if Ok.
// If the Result is Err, the error is propagated unchanged.
func Map[T any, U any](r Result[T], fn func(T) U) Result[U] {
	if r.ok {
		return Ok(fn(r.value))
	}
	return Err[U](r.err)
}

// FlatMap transforms the value inside a Result using fn that itself
// returns a Result. This enables chaining fallible operations.
func FlatMap[T any, U any](r Result[T], fn func(T) Result[U]) Result[U] {
	if r.ok {
		return fn(r.value)
	}
	return Err[U](r.err)
}

// String returns a human-readable representation of the Result.
func (r Result[T]) String() string {
	if r.ok {
		return fmt.Sprintf("Ok(%v)", r.value)
	}
	return fmt.Sprintf("Err(%v)", r.err)
}
