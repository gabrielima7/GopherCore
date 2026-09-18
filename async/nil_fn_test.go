package async

import (
	"context"
	"errors"
	"testing"
)

func TestAsyncNilFunctions_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "Go with nil func",
			fn: func() error {
				var recoveredErr error
				done := make(chan struct{})
				Go(nil, func(err error) {
					recoveredErr = err
					close(done)
				})
				<-done
				return recoveredErr
			},
		},
		{
			name: "GoErr with nil func",
			fn: func() error {
				err := <-GoErr(nil)
				return err
			},
		},
		{
			name: "Group.Go with nil func",
			fn: func() error {
				g := NewGroup()
				g.Go(nil)
				errs := g.Wait()
				if len(errs) != 1 {
					return errors.New("expected 1 error")
				}
				return errs[0]
			},
		},
		{
			name: "Map with nil func",
			fn: func() error {
				_, err := Map[int, int](context.Background(), []int{1}, 1, nil)
				return err
			},
		},
		{
			name: "Fan with nil func",
			fn: func() error {
				errs := Fan[int](context.Background(), []int{1}, nil)
				if len(errs) != 1 {
					return errors.New("expected 1 error")
				}
				return errs[0]
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			var pe *PanicError
			if !errors.As(err, &pe) {
				t.Fatalf("expected PanicError, got %v", err)
			}
		})
	}
}
