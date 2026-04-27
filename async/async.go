// Package async provides safe goroutine management utilities including
// panic recovery, fan-out/fan-in patterns, and bounded concurrent mapping.
// All functions accept context.Context for cancellation support.
package async

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

// PanicError wraps a recovered panic value with its corresponding stack trace.
//
// Purpose: This allows callers to inspect the exact location and cause of the panic
// without terminating the entire application.
// Constraints: Can be safely initialized with any panic interface{} value.
// Errors: Represents a critical runtime failure that has been caught safely.
// Thread-safety: Pure struct, safe to pass across goroutine channels.
type PanicError struct {
	Value any
	Stack string
}

// Error implements the error interface for PanicError, returning a formatted
// string containing the panic value and the full stack trace.
//
// Purpose: To satisfy the standard Go error interface.
// Constraints: Requires the PanicError struct to be populated.
// Errors: None returned directly.
// Thread-safety: Read-only access to internal fields, fully thread-safe.
func (p *PanicError) Error() string {
	return fmt.Sprintf("panic recovered: %v\n%s", p.Value, p.Stack)
}

// Go launches a new goroutine safely with automatic panic recovery.
//
// Purpose: Executes a given function fn concurrently without risking application crash.
// Constraints: The function fn must encapsulate its own logic. Optional onPanic callbacks can be provided.
// Errors: If a panic occurs, it is converted to a PanicError and passed to the callback.
// Thread-safety: Safely detaches and isolates panic propagation from the calling goroutine. Safe for concurrent use.
func Go(fn func(), onPanic ...func(err error)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr := &PanicError{
					Value: r,
					Stack: string(debug.Stack()),
				}
				if len(onPanic) > 0 && onPanic[0] != nil {
					onPanic[0](panicErr)
				}
			}
		}()
		fn()
	}()
}

// GoErr launches a new goroutine safely that executes fn and returns an error channel.
//
// Purpose: Executes fn concurrently and streams its error result back to the caller.
// Constraints: The returned channel is buffered to 1. The caller must read it or ignore it.
// Errors: Returns any error produced by fn, or a PanicError if a panic occurred.
// Thread-safety: Inherently concurrent-safe. Panics are recovered and sent safely to the channel.
func GoErr(fn func() error) <-chan error {
	ch := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- &PanicError{Value: r, Stack: string(debug.Stack())}
			}
		}()
		ch <- fn()
	}()
	return ch
}

// Group manages a collection of goroutines and collects all errors returned by them.
//
// Purpose: Structurally similar to errgroup, but includes built-in panic recovery for every launched goroutine.
// Constraints: All launched goroutines must finish before Wait() is unblocked.
// Errors: Accumulates multiple errors and panics from its constituent goroutines.
// Thread-safety: Uses internal sync primitives to be entirely safe for concurrent execution from multiple workers.
type Group struct {
	wg   sync.WaitGroup
	mu   sync.Mutex
	errs []error
}

// NewGroup creates and returns a new Group instance ready for managing
// a collection of goroutines safely.
//
// Purpose: Factory constructor for Group.
// Constraints: None.
// Errors: Never fails.
// Thread-safety: Safe for concurrent use, returns an isolated pointer instance.
func NewGroup() *Group {
	return &Group{}
}

// Go launches a goroutine within the Group to execute the provided function fn.
//
// Purpose: Adds a new task to the group. It automatically handles panic recovery.
// Constraints: Must be called before calling Wait().
// Errors: Appends returned errors and panics to the Group's internal error slice.
// Thread-safety: Concurrent accesses to the internal error slice are safely synchronized via a mutex lock.
func (g *Group) Go(fn func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				g.mu.Lock()
				g.errs = append(g.errs, &PanicError{Value: r, Stack: string(debug.Stack())})
				g.mu.Unlock()
			}
		}()
		if err := fn(); err != nil {
			g.mu.Lock()
			g.errs = append(g.errs, err)
			g.mu.Unlock()
		}
	}()
}

// Wait blocks the calling goroutine until all goroutines launched within the Group
// have completed execution.
//
// Purpose: Synchronizes and waits for all launched tasks to finish.
// Constraints: Should only be called once after all Go() calls have been issued.
// Errors: Returns a slice containing all collected errors, including any recovered panics. Returns nil if successful.
// Thread-safety: Safely accesses the internal error slice protected by a mutex lock. Safe for concurrent calls.
func (g *Group) Wait() []error {
	g.wg.Wait()
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.errs) == 0 {
		return nil
	}
	return g.errs
}

// Map applies the function fn to each item in the items slice concurrently,
// enforcing a strict bounded parallelism limit based on the concurrency parameter.
//
// Purpose: Transforms a slice of inputs into a slice of outputs concurrently using a bounded pool.
// Constraints: concurrency must be > 0 (defaults to 1 if not). Respects context cancellation.
// Errors: Returns the first encountered error (including context cancellation).
// Thread-safety: Uses a buffered channel as a semaphore to restrict concurrency. Panics are gracefully recovered. Safe for concurrent use.
func Map[T any, R any](ctx context.Context, items []T, concurrency int, fn func(context.Context, T) (R, error)) ([]R, error) {
	if concurrency <= 0 {
		concurrency = 1
	}

	results := make([]R, len(items))
	errs := make([]error, len(items))

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	// Launch workers to process items concurrently.
	for i, item := range items {
		// Fast-path context cancellation check before spawning.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Acquire semaphore slot to enforce bounded concurrency limit.
		sem <- struct{}{}
		wg.Add(1)

		go func(idx int, val T) {
			defer wg.Done()
			// Release the semaphore slot back to the channel immediately before exiting.
			defer func() { <-sem }()
			// Recover panics gracefully without bringing down the application.
			defer func() {
				if r := recover(); r != nil {
					errs[idx] = &PanicError{Value: r, Stack: string(debug.Stack())}
				}
			}()

			// Check context before executing potentially heavy operation.
			if ctx.Err() != nil {
				errs[idx] = ctx.Err()
				return
			}

			// Perform the user-provided work. Safe concurrently because each worker writes to its own index.
			result, err := fn(ctx, val)
			results[idx] = result
			errs[idx] = err
		}(i, item)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

// Fan launches the provided function fn for each item in the items slice concurrently
// with no upper bound on parallelism (unbounded concurrency).
//
// Purpose: Quickly executes a side-effecting function for each element concurrently.
// Constraints: It respects context cancellation. For bounded concurrency, prefer using Map.
// Errors: Safely collects and returns all errors encountered, including recovered panics.
// Thread-safety: Uses a sync.WaitGroup and a sync.Mutex to safely collect errors without data races.
func Fan[T any](ctx context.Context, items []T, fn func(context.Context, T) error) []error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	// Iterate through the items and launch a goroutine for each.
	for _, item := range items {
		// Stop launching new goroutines if the context is already canceled.
		if ctx.Err() != nil {
			mu.Lock()
			errs = append(errs, ctx.Err())
			mu.Unlock()
			break
		}

		wg.Add(1)
		go func(val T) {
			defer wg.Done()
			// Recover from panics inside the fan worker goroutine.
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					errs = append(errs, &PanicError{Value: r, Stack: string(debug.Stack())})
					mu.Unlock()
				}
			}()
			// Execute the worker logic and capture returned errors.
			if err := fn(ctx, val); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(item)
	}

	// Wait for all unbounded worker goroutines to finish execution.
	wg.Wait()
	return errs
}
