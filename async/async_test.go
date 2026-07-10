package async

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestGo_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		fn          func()
		onPanic     func(errCh chan error) func(error)
		expectPanic bool
		panicVal    any
	}{
		{
			name: "success",
			fn: func() {
				// successful execution
			},
			onPanic:     nil,
			expectPanic: false,
		},
		{
			name: "panic recovery with callback",
			fn: func() {
				panic("test panic")
			},
			onPanic: func(errCh chan error) func(error) {
				return func(err error) {
					errCh <- err
				}
			},
			expectPanic: true,
			panicVal:    "test panic",
		},
		{
			name: "panic silent recovery",
			fn: func() {
				panic("silent panic")
			},
			onPanic:     nil,   // No callback, should recover silently
			expectPanic: false, // We don't expect a panic on the callback channel
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doneCh := make(chan struct{})
			errCh := make(chan error, 1)

			var onPanicCb func(error)
			if tt.onPanic != nil {
				onPanicCb = tt.onPanic(errCh)
			}

			// Wrap the original function to track completion for non-panic cases
			wrappedFn := func() {
				tt.fn()
				close(doneCh) // Only executes if tt.fn() finishes cleanly without panicking
			}

			if onPanicCb != nil {
				Go(wrappedFn, onPanicCb)
			} else {
				Go(wrappedFn)
			}

			if tt.expectPanic {
				timer := time.NewTimer(time.Second)
				defer timer.Stop()
				select {
				case err := <-errCh:
					var pe *PanicError
					if !errors.As(err, &pe) {
						t.Fatalf("expected PanicError, got %T", err)
					}
					if pe.Value != tt.panicVal {
						t.Fatalf("unexpected panic value: %v", pe.Value)
					}
					if pe.Stack == "" {
						t.Fatal("expected stack trace")
					}
				case <-timer.C:
					t.Fatal("timeout waiting for panic recovery")
				}
			} else {
				timer := time.NewTimer(500 * time.Millisecond)
				defer timer.Stop()
				select {
				case <-doneCh:
					// Success or silent recovery complete
				case <-timer.C:
					if tt.name != "panic silent recovery" {
						t.Fatal("timeout waiting for goroutine")
					}
					// For silent recovery, the done channel isn't closed because the panic interrupts it,
					// but the program shouldn't crash, so a timeout is expected and OK.
				}
			}
		})
	}
}

func TestGoErr_TableDriven(t *testing.T) {
	errBoom := errors.New("boom")
	tests := []struct {
		name        string
		fn          func() error
		expectErr   error
		expectPanic bool
	}{
		{
			name:        "success",
			fn:          func() error { return nil },
			expectErr:   nil,
			expectPanic: false,
		},
		{
			name:        "error",
			fn:          func() error { return errBoom },
			expectErr:   errBoom,
			expectPanic: false,
		},
		{
			name:        "panic",
			fn:          func() error { panic("kaboom") },
			expectErr:   nil,
			expectPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := GoErr(tt.fn)
			err := <-ch

			if tt.expectPanic {
				var pe *PanicError
				if !errors.As(err, &pe) {
					t.Fatalf("expected PanicError, got %T: %v", err, err)
				}
			} else if tt.expectErr != nil {
				if !errors.Is(err, tt.expectErr) {
					if err == nil || err.Error() != tt.expectErr.Error() {
						t.Fatalf("expected '%v', got: %v", tt.expectErr, err)
					}
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGroup_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		funcs       []func() error
		expectErrs  int
		expectPanic bool
	}{
		{
			name: "success",
			funcs: []func() error{
				func() error { return nil },
				func() error { return nil },
				func() error { return nil },
			},
			expectErrs:  0,
			expectPanic: false,
		},
		{
			name: "errors",
			funcs: []func() error{
				func() error { return nil },
				func() error { return errors.New("err1") },
				func() error { return errors.New("err2") },
			},
			expectErrs:  2,
			expectPanic: false,
		},
		{
			name: "panic recovery",
			funcs: []func() error{
				func() error { panic("group panic") },
			},
			expectErrs:  1,
			expectPanic: true,
		},
		{
			name: "mixed errors and panics",
			funcs: []func() error{
				func() error { return nil },
				func() error { panic("another panic") },
				func() error { return errors.New("err3") },
			},
			expectErrs:  2,
			expectPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGroup()
			for _, fn := range tt.funcs {
				g.Go(fn)
			}

			errs := g.Wait()
			if len(errs) != tt.expectErrs {
				t.Fatalf("expected %d errors, got %d: %v", tt.expectErrs, len(errs), errs)
			}

			if tt.expectPanic {
				foundPanic := false
				for _, err := range errs {
					var pe *PanicError
					if errors.As(err, &pe) {
						foundPanic = true
					}
				}
				if !foundPanic {
					t.Fatalf("expected at least one PanicError, got none in %v", errs)
				}
			}
		})
	}
}

func TestMapSuccess(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	results, err := Map(context.Background(), items, 3, func(_ context.Context, n int) (int, error) {
		return n * 2, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{2, 4, 6, 8, 10}
	for i, v := range results {
		if v != expected[i] {
			t.Fatalf("results[%d] = %d, expected %d", i, v, expected[i])
		}
	}
}

func TestMapError(t *testing.T) {
	items := []int{1, 2, 3}
	_, err := Map(context.Background(), items, 2, func(_ context.Context, n int) (int, error) {
		if n == 2 {
			return 0, errors.New("err at 2")
		}
		return n, nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMapContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	items := []int{1, 2, 3}
	_, err := Map(ctx, items, 2, func(_ context.Context, n int) (int, error) {
		return n, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestMapPanicRecovery(t *testing.T) {
	items := []int{1, 2, 3}
	_, err := Map(context.Background(), items, 1, func(_ context.Context, n int) (int, error) {
		if n == 2 {
			panic("map panic")
		}
		return n, nil
	})
	if err == nil {
		t.Fatal("expected error from panic")
	}
	var pe *PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("expected PanicError, got %T: %v", err, err)
	}
}

func TestMapZeroConcurrency(t *testing.T) {
	items := []int{1, 2, 3}
	results, err := Map(context.Background(), items, 0, func(_ context.Context, n int) (int, error) {
		return n * 10, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, v := range results {
		if v != (i+1)*10 {
			t.Fatalf("results[%d] = %d, expected %d", i, v, (i+1)*10)
		}
	}
}

func TestMapContextCancelledInsideGoroutine(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	items := []int{1, 2, 3, 4, 5}
	_, err := Map(ctx, items, 1, func(_ context.Context, n int) (int, error) {
		if n == 2 {
			cancel()
			// The cancellation may be detected in the next goroutine launch.
			return 0, errors.New("triggered cancel")
		}
		return n, nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMapEmptySlice(t *testing.T) {
	results, err := Map(context.Background(), []int{}, 5, func(_ context.Context, n int) (int, error) {
		return n, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %d", len(results))
	}
}

func TestFanSuccess(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	var sum atomic.Int64
	errs := Fan(context.Background(), items, func(_ context.Context, n int) error {
		sum.Add(int64(n))
		return nil
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if sum.Load() != 15 {
		t.Fatalf("expected 15, got %d", sum.Load())
	}
}

func TestFanErrors(t *testing.T) {
	items := []int{1, 2, 3}
	errs := Fan(context.Background(), items, func(_ context.Context, n int) error {
		if n%2 == 0 {
			return errors.New("even error")
		}
		return nil
	})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

func TestFanContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before Fan starts.

	items := []int{1, 2, 3}
	errs := Fan(ctx, items, func(_ context.Context, n int) error {
		return nil
	})
	if len(errs) == 0 {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(errs[0], context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", errs[0])
	}
}

func TestFanPanicRecovery(t *testing.T) {
	items := []int{1, 2, 3}
	errs := Fan(context.Background(), items, func(_ context.Context, n int) error {
		if n == 2 {
			panic("fan panic")
		}
		return nil
	})

	foundPanic := false
	for _, err := range errs {
		var pe *PanicError
		if errors.As(err, &pe) {
			foundPanic = true
		}
	}
	if !foundPanic {
		t.Fatal("expected at least one PanicError from Fan")
	}
}

func TestPanicErrorString(t *testing.T) {
	pe := &PanicError{
		Value: "test",
		Stack: "stack trace here",
	}
	s := pe.Error()
	if s == "" {
		t.Fatal("expected non-empty error string")
	}
}

func TestMap_TableDriven(t *testing.T) {
	errTest := errors.New("test error")

	tests := []struct {
		name        string
		items       []int
		concurrency int
		fn          func(context.Context, int) (int, error)
		ctxCancelFn func() (context.Context, context.CancelFunc)
		expectErr   error
		expectPanic bool
	}{
		{
			name:        "nil slice",
			items:       nil,
			concurrency: 2,
			fn:          func(ctx context.Context, n int) (int, error) { return n, nil },
			expectErr:   nil,
		},
		{
			name:        "empty slice",
			items:       []int{},
			concurrency: 2,
			fn:          func(ctx context.Context, n int) (int, error) { return n, nil },
			expectErr:   nil,
		},
		{
			name:        "invalid concurrency defaults to 1",
			items:       []int{1, 2, 3},
			concurrency: -5,
			fn:          func(ctx context.Context, n int) (int, error) { return n * 2, nil },
			expectErr:   nil,
		},
		{
			name:        "worker returns context canceled error while context is still active",
			items:       []int{1, 2, 3},
			concurrency: 1,
			fn:          func(ctx context.Context, n int) (int, error) { return 0, context.Canceled },
			expectErr:   context.Canceled,
		},
		{
			name:        "immediate context cancellation",
			items:       []int{1, 2, 3},
			concurrency: 2,
			fn:          func(ctx context.Context, n int) (int, error) { return n, nil },
			ctxCancelFn: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			expectErr: context.Canceled,
		},
		{
			name:        "worker error",
			items:       []int{1, 2, 3},
			concurrency: 2,
			fn: func(ctx context.Context, n int) (int, error) {
				if n == 2 {
					return 0, errTest
				}
				return n, nil
			},
			expectErr: errTest,
		},
		{
			name:        "worker panic",
			items:       []int{1, 2, 3},
			concurrency: 2,
			fn: func(ctx context.Context, n int) (int, error) {
				if n == 2 {
					panic("test panic")
				}
				return n, nil
			},
			expectPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var cancel context.CancelFunc
			if tt.ctxCancelFn != nil {
				ctx, cancel = tt.ctxCancelFn()
				defer cancel()
			}

			_, err := Map(ctx, tt.items, tt.concurrency, tt.fn)
			if tt.expectPanic {
				var pe *PanicError
				if !errors.As(err, &pe) {
					t.Errorf("expected PanicError, got %v", err)
				}
			} else if tt.expectErr != nil {
				if !errors.Is(err, tt.expectErr) {
					t.Errorf("expected error %v, got %v", tt.expectErr, err)
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestFan_TableDriven(t *testing.T) {
	errTest := errors.New("test error")

	tests := []struct {
		name        string
		items       []int
		fn          func(context.Context, int) error
		ctxCancelFn func() (context.Context, context.CancelFunc)
		expectErrs  int
		expectPanic bool
	}{
		{
			name:       "nil slice",
			items:      nil,
			fn:         func(ctx context.Context, n int) error { return nil },
			expectErrs: 0,
		},
		{
			name:       "empty slice",
			items:      []int{},
			fn:         func(ctx context.Context, n int) error { return nil },
			expectErrs: 0,
		},
		{
			name:  "immediate context cancellation",
			items: []int{1, 2, 3},
			fn:    func(ctx context.Context, n int) error { return nil },
			ctxCancelFn: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			expectErrs: 1, // fan appends ctx.Err() exactly once upon detecting cancellation in launch loop
		},
		{
			name:  "worker errors",
			items: []int{1, 2, 3, 4},
			fn: func(ctx context.Context, n int) error {
				if n%2 == 0 {
					return errTest
				}
				return nil
			},
			expectErrs: 2,
		},
		{
			name:  "worker panic",
			items: []int{1, 2, 3},
			fn: func(ctx context.Context, n int) error {
				if n == 2 {
					panic("test panic")
				}
				return nil
			},
			expectErrs:  1,
			expectPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var cancel context.CancelFunc
			if tt.ctxCancelFn != nil {
				ctx, cancel = tt.ctxCancelFn()
				defer cancel()
			}

			errs := Fan(ctx, tt.items, tt.fn)
			if len(errs) != tt.expectErrs {
				t.Errorf("expected %d errors, got %d", tt.expectErrs, len(errs))
			}

			if tt.expectPanic {
				foundPanic := false
				for _, err := range errs {
					var pe *PanicError
					if errors.As(err, &pe) {
						foundPanic = true
					}
				}
				if !foundPanic {
					t.Errorf("expected at least one PanicError, got none")
				}
			}
		})
	}
}

type customCtxForTest struct {
	context.Context
	counter int32
}

func (c *customCtxForTest) Err() error {
	if atomic.AddInt32(&c.counter, 1) > 2 {
		return context.Canceled
	}
	return nil
}

func (c *customCtxForTest) Done() <-chan struct{} {
	return nil // Never done to prevent select block from matching ctx.Done()
}

func TestMapContextCancelledInsideGoroutineUncovered(t *testing.T) {
	ctx := &customCtxForTest{Context: context.Background()}
	items := []int{1, 2}
	results, err := Map(ctx, items, 2, func(c context.Context, n int) (int, error) {
		return n, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v (results: %v)", err, results)
	}
}

func TestMapWorkerReturnsContextCanceledWithActiveContext(t *testing.T) {
	items := []int{1}
	results, err := Map(context.Background(), items, 1, func(c context.Context, n int) (int, error) {
		return 0, context.Canceled
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v (results: %v)", err, results)
	}
}

func TestMapContextCancellationDuringSemaphoreWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a channel to coordinate the test. We want the first worker to start
	// and hold the semaphore, and the second worker to block on the semaphore.
	workerStarted := make(chan struct{})

	items := []int{1, 2}

	go func() {
		// Wait until the first worker has started and grabbed the semaphore.
		<-workerStarted
		// Yield slightly to ensure the second loop iteration hits the select block
		// and blocks on the semaphore rather than the fast-path ctx.Err().
		time.Sleep(10 * time.Millisecond)
		// Cancel the context to unblock the second worker's select statement.
		cancel()
	}()

	results, err := Map(ctx, items, 1, func(c context.Context, n int) (int, error) {
		if n == 1 {
			close(workerStarted)
			// Block until context is cancelled to hold the semaphore.
			<-c.Done()
			return 0, c.Err()
		}
		return n, nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v (results: %v)", err, results)
	}
}
