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

// PanicError wraps a recovered panic value with its stack trace.
type PanicError struct {
	Value any
	Stack string
}

// Error implements the error interface.
func (p *PanicError) Error() string {
	return fmt.Sprintf("panic recovered: %v\n%s", p.Value, p.Stack)
}

// Go launches a goroutine with automatic panic recovery.
// If the goroutine panics, the panic is captured and passed to
// the optional onPanic callback. If no callback is provided,
// the panic is silently recovered.
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

// GoErr launches a goroutine that returns an error. The result is
// sent to the returned channel. Panics are recovered and returned as errors.
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

// Group manages a collection of goroutines and collects their errors.
// It is similar to errgroup but with panic recovery built in.
type Group struct {
	wg   sync.WaitGroup
	mu   sync.Mutex
	errs []error
}

// NewGroup creates a new Group.
func NewGroup() *Group {
	return &Group{}
}

// Go launches a goroutine within the Group with panic recovery.
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

// Wait blocks until all goroutines in the Group have completed.
// Returns all collected errors (nil if no errors occurred).
func (g *Group) Wait() []error {
	g.wg.Wait()
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.errs) == 0 {
		return nil
	}
	return g.errs
}

// Map applies fn to each item in items concurrently with bounded parallelism.
// It respects context cancellation and recovers from panics.
func Map[T any, R any](ctx context.Context, items []T, concurrency int, fn func(context.Context, T) (R, error)) ([]R, error) {
	if concurrency <= 0 {
		concurrency = 1
	}

	results := make([]R, len(items))
	errs := make([]error, len(items))

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, item := range items {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(idx int, val T) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					errs[idx] = &PanicError{Value: r, Stack: string(debug.Stack())}
				}
			}()

			if ctx.Err() != nil {
				errs[idx] = ctx.Err()
				return
			}

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

// Fan launches fn for each item in items concurrently with no bound.
// Use Map for bounded concurrency.
func Fan[T any](ctx context.Context, items []T, fn func(context.Context, T) error) []error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for _, item := range items {
		if ctx.Err() != nil {
			mu.Lock()
			errs = append(errs, ctx.Err())
			mu.Unlock()
			break
		}

		wg.Add(1)
		go func(val T) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					errs = append(errs, &PanicError{Value: r, Stack: string(debug.Stack())})
					mu.Unlock()
				}
			}()
			if err := fn(ctx, val); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(item)
	}

	wg.Wait()
	return errs
}
