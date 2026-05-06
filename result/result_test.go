package result

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
)

func TestOk(t *testing.T) {
	r := Ok(42)
	if !r.IsOk() {
		t.Fatal("expected Ok")
	}
	if r.IsErr() {
		t.Fatal("expected not Err")
	}
	val, err := r.Unwrap()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Fatalf("expected 42, got %d", val)
	}
}

func TestErr(t *testing.T) {
	e := errors.New("something failed")
	r := Err[int](e)
	if r.IsOk() {
		t.Fatal("expected Err")
	}
	if !r.IsErr() {
		t.Fatal("expected IsErr to be true")
	}
	_, err := r.Unwrap()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "something failed" {
		t.Fatalf("expected 'something failed', got %q", err.Error())
	}
}

func TestErrf(t *testing.T) {
	r := Errf[string]("failed with code %d", 404)
	if r.IsOk() {
		t.Fatal("expected Err")
	}
	if r.Error().Error() != "failed with code 404" {
		t.Fatalf("unexpected error message: %v", r.Error())
	}
}

func TestOf(t *testing.T) {
	tests := []struct {
		name     string
		valFn    func() (int, error)
		expected int
		wantErr  bool
	}{
		{
			name:     "success",
			valFn:    func() (int, error) { return strconv.Atoi("42") },
			expected: 42,
			wantErr:  false,
		},
		{
			name:     "failure",
			valFn:    func() (int, error) { return strconv.Atoi("not_a_number") },
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Of(tt.valFn())
			if tt.wantErr {
				if !r.IsErr() {
					t.Errorf("expected Err for %s", tt.name)
				}
			} else {
				if !r.IsOk() {
					t.Errorf("expected Ok for %s", tt.name)
				}
				val, _ := r.Unwrap()
				if val != tt.expected {
					t.Errorf("expected %d, got %d for %s", tt.expected, val, tt.name)
				}
			}
		})
	}
}

func TestUnwrapOr(t *testing.T) {
	ok := Ok(10)
	if ok.UnwrapOr(0) != 10 {
		t.Fatal("expected 10")
	}
	fail := Err[int](errors.New("err"))
	if fail.UnwrapOr(99) != 99 {
		t.Fatal("expected fallback 99")
	}
}

func TestUnwrapOrElse(t *testing.T) {
	tests := []struct {
		name     string
		res      Result[int]
		fn       func(error) int
		expected int
	}{
		{
			name:     "success yields value",
			res:      Ok(10),
			fn:       func(_ error) int { return -1 },
			expected: 10,
		},
		{
			name: "failure invokes fallback func",
			res:  Err[int](errors.New("boom")),
			fn: func(err error) int {
				if err.Error() == "boom" {
					return -1
				}
				return 0
			},
			expected: -1,
		},
		{
			name: "failure with nil error explicitly handled",
			res:  Err[int](nil),
			fn: func(err error) int {
				if err == nil {
					return 99
				}
				return 0
			},
			expected: 99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.res.UnwrapOrElse(tt.fn)
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name string
		res  Result[int]
		err  error
	}{
		{
			name: "success has nil error",
			res:  Ok(42),
			err:  nil,
		},
		{
			name: "failure has error",
			res:  Err[int](errors.New("failed")),
			err:  errors.New("failed"),
		},
		{
			name: "failure with nil error",
			res:  Err[int](nil),
			err:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.res.Error()
			if tt.err == nil {
				if err != nil {
					t.Errorf("expected nil error, got %v", err)
				}
			} else {
				if err == nil || err.Error() != tt.err.Error() {
					t.Errorf("expected %v error, got %v", tt.err, err)
				}
			}
		})
	}
}

func TestMap(t *testing.T) {
	r := Ok(5)
	doubled := Map(r, func(v int) int { return v * 2 })
	val, err := doubled.Unwrap()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 10 {
		t.Fatalf("expected 10, got %d", val)
	}

	fail := Err[int](errors.New("err"))
	mapped := Map(fail, func(v int) string { return fmt.Sprintf("%d", v) })
	if mapped.IsOk() {
		t.Fatal("expected Err propagation")
	}
}

func TestFlatMap(t *testing.T) {
	r := Ok(10)
	halved := FlatMap(r, func(v int) Result[int] {
		if v%2 != 0 {
			return Err[int](errors.New("odd number"))
		}
		return Ok(v / 2)
	})
	val, err := halved.Unwrap()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 5 {
		t.Fatalf("expected 5, got %d", val)
	}

	fail := Err[int](errors.New("initial"))
	chained := FlatMap(fail, func(v int) Result[int] { return Ok(v) })
	if chained.IsOk() {
		t.Fatal("expected Err propagation")
	}
}

func TestString(t *testing.T) {
	ok := Ok(42)
	if ok.String() != "Ok(42)" {
		t.Fatalf("unexpected string: %s", ok.String())
	}
	fail := Err[int](errors.New("boom"))
	if fail.String() != "Err(boom)" {
		t.Fatalf("unexpected string: %s", fail.String())
	}
}

func FuzzResultUnwrapOr(f *testing.F) {
	f.Add(42, 0)
	f.Add(0, -1)
	f.Add(-100, 100)
	f.Fuzz(func(t *testing.T, value int, fallback int) {
		ok := Ok(value)
		if ok.UnwrapOr(fallback) != value {
			t.Fatalf("Ok.UnwrapOr should return value, got %d", ok.UnwrapOr(fallback))
		}
		fail := Err[int](errors.New("err"))
		if fail.UnwrapOr(fallback) != fallback {
			t.Fatalf("Err.UnwrapOr should return fallback, got %d", fail.UnwrapOr(fallback))
		}
	})
}

func TestResultConcurrency(t *testing.T) {
	tests := []struct {
		name  string
		isMap bool
		mapFn func(int, int) Result[int]
	}{
		{
			name:  "Map concurrent execution",
			isMap: true,
			mapFn: func(base, val int) Result[int] {
				return Map(Ok(base), func(v int) int { return v + val })
			},
		},
		{
			name:  "FlatMap concurrent execution",
			isMap: false,
			mapFn: func(base, val int) Result[int] {
				return FlatMap(Ok(base), func(v int) Result[int] { return Ok(v * val) })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const numGoroutines = 100
			errCh := make(chan error, numGoroutines)
			for i := 0; i < numGoroutines; i++ {
				go func(val int) {
					res, err := tt.mapFn(10, val).Unwrap()
					if err != nil {
						errCh <- err
						return
					}
					expected := 10 + val
					if !tt.isMap {
						expected = 10 * val
					}
					if res != expected {
						errCh <- fmt.Errorf("result mismatch: expected %d, got %d", expected, res)
						return
					}
					errCh <- nil
				}(i)
			}
			for i := 0; i < numGoroutines; i++ {
				if err := <-errCh; err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
