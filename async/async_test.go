package async

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestGoSuccess(t *testing.T) {
	done := make(chan struct{})
	Go(func() {
		close(done)
	})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for goroutine")
	}
}

func TestGoPanicRecovery(t *testing.T) {
	panicCh := make(chan error, 1)
	Go(func() {
		panic("test panic")
	}, func(err error) {
		panicCh <- err
	})
	select {
	case err := <-panicCh:
		var pe *PanicError
		if !errors.As(err, &pe) {
			t.Fatalf("expected PanicError, got %T", err)
		}
		if pe.Value != "test panic" {
			t.Fatalf("unexpected panic value: %v", pe.Value)
		}
		if pe.Stack == "" {
			t.Fatal("expected stack trace")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for panic recovery")
	}
}

func TestGoPanicSilentRecovery(t *testing.T) {
	// No onPanic callback — should recover silently without crashing.
	done := make(chan struct{})
	Go(func() {
		defer close(done)
		panic("silent panic")
	})
	select {
	case <-done:
		// This won't be called since panic prevents defer from closing 'done' in fn,
		// but the goroutine should not crash the process.
	case <-time.After(500 * time.Millisecond):
		// Expected: goroutine recovered silently.
	}
}

func TestGoErr(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ch := GoErr(func() error { return nil })
		err := <-ch
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("error", func(t *testing.T) {
		ch := GoErr(func() error { return errors.New("boom") })
		err := <-ch
		if err == nil || err.Error() != "boom" {
			t.Fatalf("expected 'boom', got: %v", err)
		}
	})
	t.Run("panic", func(t *testing.T) {
		ch := GoErr(func() error { panic("kaboom") })
		err := <-ch
		var pe *PanicError
		if !errors.As(err, &pe) {
			t.Fatalf("expected PanicError, got %T: %v", err, err)
		}
	})
}

func TestGroupSuccess(t *testing.T) {
	g := NewGroup()
	var count atomic.Int32

	for i := 0; i < 10; i++ {
		g.Go(func() error {
			count.Add(1)
			return nil
		})
	}

	errs := g.Wait()
	if errs != nil {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if count.Load() != 10 {
		t.Fatalf("expected 10, got %d", count.Load())
	}
}

func TestGroupErrors(t *testing.T) {
	g := NewGroup()

	g.Go(func() error { return nil })
	g.Go(func() error { return errors.New("err1") })
	g.Go(func() error { return errors.New("err2") })

	errs := g.Wait()
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errs))
	}
}

func TestGroupPanicRecovery(t *testing.T) {
	g := NewGroup()
	g.Go(func() error { panic("group panic") })

	errs := g.Wait()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error from panic, got %d", len(errs))
	}
	var pe *PanicError
	if !errors.As(errs[0], &pe) {
		t.Fatalf("expected PanicError, got %T", errs[0])
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
