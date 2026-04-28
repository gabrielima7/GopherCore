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
// Thread-safety: Pure struct, safe to pass across goroutine channels.
type PanicError struct {
	Value any
	Stack string
}

// Error implements the error interface for PanicError, returning a formatted
// string containing the panic value and the full stack trace.
// Thread-safety: Read-only access to internal fields.
func (p *PanicError) Error() string {
	return fmt.Sprintf("panic recovered: %v\n%s", p.Value, p.Stack)
}

// Go launches a new goroutine safely with automatic panic recovery.
//
// Constraints: If the provided function fn panics during execution, the panic is gracefully
// caught and converted into a PanicError. This error is then passed to the optional onPanic
// callback functions, if any are provided. If no callback is provided, the panic is silently recovered.
// Thread-safety: Safely detaches and isolates panic propagation from the calling goroutine.
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
// Constraints: The result of fn is sent to the returned channel, which is buffered to prevent
// goroutine leaks if the caller does not read from it immediately.
// Thread-safety: If the goroutine panics, the panic is recovered and sent safely to the
// channel as a PanicError. Channel access is inherently concurrent-safe.
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
// Purpose: It is structurally similar to golang.org/x/sync/errgroup but natively includes
// built-in panic recovery for every launched goroutine.
// Thread-safety: It uses internal sync primitives to be entirely safe for concurrent
// execution and error collection from multiple workers.
type Group struct {
	wg   sync.WaitGroup
	mu   sync.Mutex
	errs []error
}

// NewGroup creates and returns a new Group instance ready for managing
// a collection of goroutines safely.
// Constraints: Instantiates without arguments, assumes unbounded slice allocation.
// Thread-safety: Returns a new struct instance pointer. Safe to share.
func NewGroup() *Group {
	return &Group{}
}

// Go launches a goroutine within the Group to execute the provided function fn.
//
// Purpose: It automatically handles panic recovery by capturing the panic and appending
// it to the Group's internal error slice as a PanicError.
// Thread-safety: All concurrent accesses to the internal error slice are safely
// synchronized via a mutex lock to strictly prevent data races.
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
// Constraints: It returns a slice containing all collected errors, including any recovered panics.
// If no errors occurred, it returns nil.
// Thread-safety: It safely accesses the internal error slice protected by a mutex lock.
// Safe for concurrent calls, though typically called once by a single coordinator goroutine.
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
// Constraints: It respects context cancellation, immediately halting further processing if
// the context is canceled, returning ctx.Err(). It returns the mapped results in the exact
// same order as the input items, or the first encountered error (including context cancellation).
// Thread-safety: It uses a buffered channel as a counting semaphore to restrict the number of
// concurrently active goroutines. Panics within workers are gracefully recovered.
// Each worker writes its result to its own unique pre-allocated slice index, avoiding race conditions entirely.
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
			// Recover panics gracefully without bringing down the application,
			// converting the recovered panic value into a PanicError with stack trace.
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

			// Perform the user-provided work. Safe concurrently because each worker
			// writes its result to its own unique index in the pre-allocated slice.
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
// Constraints: It respects context cancellation, aborting the launch loop early if the context is
// canceled. It safely collects and returns all errors encountered, including recovered panics.
// For bounded concurrency, prefer using Map.
// Thread-safety: It uses a sync.WaitGroup to coordinate completion and a sync.Mutex to
// safely collect any errors encountered without data races.
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
