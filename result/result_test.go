package result

import (
	"errors"
	"fmt"
	"go.uber.org/goleak"
	"strconv"
	"testing"
)

func TestResultConstructors_TableDriven(t *testing.T) {
	defer goleak.VerifyNone(t)
	errFail := errors.New("something failed")

	tests := []struct {
		name         string
		constructor  func() Result[any]
		expectOk     bool
		expectValue  any
		expectErrStr string
	}{
		{
			name: "Ok Constructor",
			constructor: func() Result[any] {
				return Ok[any](42)
			},
			expectOk:    true,
			expectValue: 42,
		},
		{
			name: "Err Constructor",
			constructor: func() Result[any] {
				return Err[any](errFail)
			},
			expectOk:     false,
			expectErrStr: "something failed",
		},
		{
			name: "Errf Constructor",
			constructor: func() Result[any] {
				return Errf[any]("failed with code %d", 404)
			},
			expectOk:     false,
			expectErrStr: "failed with code 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.constructor()

			if r.IsOk() != tt.expectOk {
				t.Fatalf("expected IsOk %v, got %v", tt.expectOk, r.IsOk())
			}
			if r.IsErr() == tt.expectOk {
				t.Fatalf("expected IsErr %v, got %v", !tt.expectOk, r.IsErr())
			}

			val, err := r.Unwrap()
			if tt.expectOk {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if val != tt.expectValue {
					t.Fatalf("expected value %v, got %v", tt.expectValue, val)
				}
				if r.Error() != nil {
					t.Fatalf("expected Error() to return nil, got %v", r.Error())
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if err.Error() != tt.expectErrStr {
					t.Fatalf("expected error string %q, got %q", tt.expectErrStr, err.Error())
				}
				if r.Error() == nil || r.Error().Error() != tt.expectErrStr {
					t.Fatalf("expected Error() string %q, got %v", tt.expectErrStr, r.Error())
				}
			}
		})
	}
}

func TestErrf(t *testing.T) {
	defer goleak.VerifyNone(t)
	tests := []struct {
		name   string
		format string
		args   []any
		expect string
	}{
		{"simple", "error %d", []any{404}, "error 404"},
		{"string", "msg: %s", []any{"not found"}, "msg: not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Errf[int](tt.format, tt.args...)
			if r.IsOk() {
				t.Fatal("expected Errf to return an Err")
			}
			if r.Error().Error() != tt.expect {
				t.Fatalf("expected error message %q, got %q", tt.expect, r.Error().Error())
			}
		})
	}
}

func TestOf(t *testing.T) {
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
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
	defer goleak.VerifyNone(t)
	ok := Ok(42)
	if ok.String() != "Ok(42)" {
		t.Fatalf("unexpected string: %s", ok.String())
	}
	fail := Err[int](errors.New("boom"))
	if fail.String() != "Err(boom)" {
		t.Fatalf("unexpected string: %s", fail.String())
	}
}

func TestResult_Methods_TableDriven(t *testing.T) {
	defer goleak.VerifyNone(t)
	errBoom := errors.New("boom")

	tests := []struct {
		name         string
		res          Result[int]
		op           string // Map, FlatMap, UnwrapOr, UnwrapOrElse, IsOk, IsErr
		fallbackVal  int
		fallbackFn   func(error) int
		mapFn        func(int) int
		flatMapFn    func(int) Result[int]
		expectedOk   bool
		expectedVal  int
		expectedErr  error
		expectedBool bool // for IsOk/IsErr
	}{
		{
			name:        "Map over Ok",
			res:         Ok(5),
			op:          "Map",
			mapFn:       func(v int) int { return v * 2 },
			expectedOk:  true,
			expectedVal: 10,
		},
		{
			name:        "Map over Err propagates error",
			res:         Err[int](errBoom),
			op:          "Map",
			mapFn:       func(v int) int { return v * 2 },
			expectedOk:  false,
			expectedErr: errBoom,
		},
		{
			name:        "FlatMap over Ok returning Ok",
			res:         Ok(10),
			op:          "FlatMap",
			flatMapFn:   func(v int) Result[int] { return Ok(v / 2) },
			expectedOk:  true,
			expectedVal: 5,
		},
		{
			name:        "FlatMap over Ok returning Err",
			res:         Ok(10),
			op:          "FlatMap",
			flatMapFn:   func(v int) Result[int] { return Err[int](errBoom) },
			expectedOk:  false,
			expectedErr: errBoom,
		},
		{
			name:        "FlatMap over Err propagates initial error",
			res:         Err[int](errBoom),
			op:          "FlatMap",
			flatMapFn:   func(v int) Result[int] { return Ok(v * 2) },
			expectedOk:  false,
			expectedErr: errBoom,
		},
		{
			name:        "Unwrap on Ok returns value",
			res:         Ok(42),
			op:          "Unwrap",
			expectedOk:  true,
			expectedVal: 42,
		},
		{
			name:        "Unwrap on Err returns error",
			res:         Err[int](errBoom),
			op:          "Unwrap",
			expectedOk:  false,
			expectedErr: errBoom,
		},
		{
			name:        "UnwrapOr on Ok returns value",
			res:         Ok(42),
			op:          "UnwrapOr",
			fallbackVal: 99,
			expectedVal: 42,
		},
		{
			name:        "UnwrapOr on Err returns fallback",
			res:         Err[int](errBoom),
			op:          "UnwrapOr",
			fallbackVal: 99,
			expectedVal: 99,
		},
		{
			name:        "UnwrapOrElse on Ok returns value",
			res:         Ok(42),
			op:          "UnwrapOrElse",
			fallbackFn:  func(e error) int { return 99 },
			expectedVal: 42,
		},
		{
			name:        "UnwrapOrElse on Err invokes fallback function",
			res:         Err[int](errBoom),
			op:          "UnwrapOrElse",
			fallbackFn:  func(e error) int { return 99 },
			expectedVal: 99,
		},
		{
			name:         "IsOk returns true for Ok",
			res:          Ok(1),
			op:           "IsOk",
			expectedBool: true,
		},
		{
			name:         "IsOk returns false for Err",
			res:          Err[int](errBoom),
			op:           "IsOk",
			expectedBool: false,
		},
		{
			name:         "IsErr returns true for Err",
			res:          Err[int](errBoom),
			op:           "IsErr",
			expectedBool: true,
		},
		{
			name:         "IsErr returns false for Ok",
			res:          Ok(1),
			op:           "IsErr",
			expectedBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.op {
			case "Map":
				mapped := Map(tt.res, tt.mapFn)
				if mapped.IsOk() != tt.expectedOk {
					t.Errorf("Map expected Ok=%v, got %v", tt.expectedOk, mapped.IsOk())
				}
				if tt.expectedOk {
					val, _ := mapped.Unwrap()
					if val != tt.expectedVal {
						t.Errorf("Map expected value %d, got %d", tt.expectedVal, val)
					}
				} else {
					if !errors.Is(mapped.Error(), tt.expectedErr) {
						t.Errorf("Map expected error %v, got %v", tt.expectedErr, mapped.Error())
					}
				}
			case "FlatMap":
				flatMapped := FlatMap(tt.res, tt.flatMapFn)
				if flatMapped.IsOk() != tt.expectedOk {
					t.Errorf("FlatMap expected Ok=%v, got %v", tt.expectedOk, flatMapped.IsOk())
				}
				if tt.expectedOk {
					val, _ := flatMapped.Unwrap()
					if val != tt.expectedVal {
						t.Errorf("FlatMap expected value %d, got %d", tt.expectedVal, val)
					}
				} else {
					if !errors.Is(flatMapped.Error(), tt.expectedErr) {
						t.Errorf("FlatMap expected error %v, got %v", tt.expectedErr, flatMapped.Error())
					}
				}
			case "UnwrapOr":
				val := tt.res.UnwrapOr(tt.fallbackVal)
				if val != tt.expectedVal {
					t.Errorf("UnwrapOr expected %d, got %d", tt.expectedVal, val)
				}
			case "Unwrap":
				val, err := tt.res.Unwrap()
				if tt.expectedOk {
					if err != nil {
						t.Errorf("Unwrap expected no error, got %v", err)
					}
					if val != tt.expectedVal {
						t.Errorf("Unwrap expected value %d, got %d", tt.expectedVal, val)
					}
				} else {
					if !errors.Is(err, tt.expectedErr) {
						t.Errorf("Unwrap expected error %v, got %v", tt.expectedErr, err)
					}
				}
			case "UnwrapOrElse":
				val := tt.res.UnwrapOrElse(tt.fallbackFn)
				if val != tt.expectedVal {
					t.Errorf("UnwrapOrElse expected %d, got %d", tt.expectedVal, val)
				}
			case "IsOk":
				if tt.res.IsOk() != tt.expectedBool {
					t.Errorf("IsOk expected %v, got %v", tt.expectedBool, tt.res.IsOk())
				}
			case "IsErr":
				if tt.res.IsErr() != tt.expectedBool {
					t.Errorf("IsErr expected %v, got %v", tt.expectedBool, tt.res.IsErr())
				}
			default:
				t.Fatalf("unknown op: %s", tt.op)
			}
		})
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
	defer goleak.VerifyNone(t)
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
