package guard

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-playground/validator/v10"
)

type createUserInput struct {
	Name  string `validate:"required,min=2,max=100"`
	Email string `validate:"required,email"`
	Age   int    `validate:"gte=0,lte=150"`
}

func TestValidate_TableDriven(t *testing.T) {
	type Inner struct {
		Code string `validate:"required,min=3"`
	}
	type Outer struct {
		Data Inner `validate:"required"`
	}
	type hidden struct {
		Exported   string `validate:"required"`
		unexported string `validate:"required"`
	}

	ptrInput := &createUserInput{Name: "Dave", Email: "dave@example.com", Age: 40}

	tests := []struct {
		name                 string
		input                any
		expectErr            bool
		expectValidationErrs bool
		expectedErrCount     int
		expectedTags         map[string]string // map field name to expected tag failure
	}{
		{
			name: "success",
			input: createUserInput{
				Name:  "Alice",
				Email: "alice@example.com",
				Age:   30,
			},
			expectErr: false,
		},
		{
			name: "failure multiple errors",
			input: createUserInput{
				Name:  "",
				Email: "not-an-email",
				Age:   -1,
			},
			expectErr:            true,
			expectValidationErrs: true,
			expectedErrCount:     3,
		},
		{
			name: "failure required only",
			input: createUserInput{
				Name:  "",
				Email: "valid@example.com",
				Age:   25,
			},
			expectErr:            true,
			expectValidationErrs: true,
			expectedErrCount:     1,
			expectedTags:         map[string]string{"Name": "required"},
		},
		{
			name:                 "non-struct input",
			input:                "not a struct",
			expectErr:            true,
			expectValidationErrs: false,
		},
		{
			name:                 "nil input",
			input:                nil,
			expectErr:            true,
			expectValidationErrs: false,
		},
		{
			name: "pointer to struct",
			input: &createUserInput{
				Name:  "Charlie",
				Email: "charlie@example.com",
				Age:   35,
			},
			expectErr: false,
		},
		{
			name:      "deeply nested struct valid",
			input:     Outer{Data: Inner{Code: "ABC"}},
			expectErr: false,
		},
		{
			name:                 "deeply nested struct invalid",
			input:                Outer{Data: Inner{Code: "A"}},
			expectErr:            true,
			expectValidationErrs: true,
			expectedErrCount:     1,
			expectedTags:         map[string]string{"Code": "min"},
		},
		{
			name:      "unexported fields ignored",
			input:     hidden{Exported: "Visible", unexported: "secret"},
			expectErr: false,
		},
		{
			name:                 "pointer to pointer to struct",
			input:                &ptrInput,
			expectErr:            true,
			expectValidationErrs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.input)
			if !tt.expectErr {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error but got nil")
			}

			var ve ValidationErrors
			isValidationErrs := errors.As(err, &ve)

			if tt.expectValidationErrs {
				if !isValidationErrs {
					t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
				}
				if len(ve) != tt.expectedErrCount {
					t.Fatalf("expected %d errors, got %d: %v", tt.expectedErrCount, len(ve), ve)
				}
				if tt.expectedTags != nil {
					for field, expectedTag := range tt.expectedTags {
						found := false
						for _, e := range ve {
							if e.Field == field && e.Tag == expectedTag {
								found = true
								break
							}
						}
						if !found {
							t.Fatalf("expected validation error for field %s with tag %s, but not found in %v", field, expectedTag, ve)
						}
					}
				}
			} else {
				if isValidationErrs {
					t.Fatalf("expected standard error, got ValidationErrors")
				}
			}
		})
	}
}

func TestValidationErrorsString(t *testing.T) {
	errs := ValidationErrors{
		{Field: "Name", Tag: "required", Message: "field 'Name' is required"},
		{Field: "Email", Tag: "email", Message: "field 'Email' must be a valid email address"},
	}
	s := errs.Error()
	if s == "" {
		t.Fatal("expected non-empty error string")
	}
	if s != "field 'Name' is required; field 'Email' must be a valid email address" {
		t.Fatalf("unexpected error string: %s", s)
	}
}

func TestValidationErrorString(t *testing.T) {
	e := ValidationError{
		Field:   "Name",
		Tag:     "required",
		Message: "field 'Name' is required",
	}
	if e.Error() != "field 'Name' is required" {
		t.Fatalf("unexpected: %s", e.Error())
	}
}

func TestRegisterValidation(t *testing.T) {
	err := RegisterValidation("is_even", func(fl validator.FieldLevel) bool {
		return fl.Field().Int()%2 == 0
	})
	if err != nil {
		t.Fatalf("unexpected error registering validation: %v", err)
	}

	type input struct {
		Value int `validate:"is_even"`
	}

	// Valid: even number.
	if err := Validate(input{Value: 4}); err != nil {
		t.Fatalf("expected 4 to be valid: %v", err)
	}

	// Invalid: odd number.
	err = Validate(input{Value: 3})
	if err == nil {
		t.Fatal("expected validation error for odd number")
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal", "hello world", "hello world"},
		{"trim whitespace", "  hello  ", "hello"},
		{"remove null bytes", "hello\x00world", "helloworld"},
		{"preserve newlines", "hello\nworld", "hello\nworld"},
		{"preserve tabs", "hello\tworld", "hello\tworld"},
		{"preserve carriage return", "hello\rworld", "hello\rworld"},
		{"empty string", "", ""},
		{"only control chars", "\x01\x02\x03", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeString(tt.input)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no html", "hello world", "hello world"},
		{"simple tag", "<b>bold</b>", "bold"},
		{"script tag", "<script>alert('xss')</script>", ""},
		{"nested", "<div><p>text</p></div>", "text"},
		{"empty", "", ""},
		{"attributes", `<a href="url">link</a>`, "link"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripHTML(tt.input)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFormatValidationErrorAllTags_TableDriven(t *testing.T) {
	// Define exactly one failing tag condition per struct to cleanly test `formatValidationError`.
	type reqInput struct {
		Val string `validate:"required"`
	}
	type emailInput struct {
		Val string `validate:"email"`
	}
	type minInput struct {
		Val string `validate:"min=5"`
	}
	type maxInput struct {
		Val string `validate:"max=3"`
	}
	type gteInput struct {
		Val int `validate:"gte=10"`
	}
	type lteInput struct {
		Val int `validate:"lte=10"`
	}
	type urlInput struct {
		Val string `validate:"url"`
	}
	type uuidInput struct {
		Val string `validate:"uuid"`
	}
	type defaultInput struct {
		Val string `validate:"alpha"`
	}

	tests := []struct {
		name          string
		input         any
		expectedTag   string
		expectedError string
	}{
		{
			name:          "required",
			input:         reqInput{},
			expectedTag:   "required",
			expectedError: "field 'Val' is required",
		},
		{
			name:          "email",
			input:         emailInput{Val: "not-email"},
			expectedTag:   "email",
			expectedError: "field 'Val' must be a valid email address",
		},
		{
			name:          "min",
			input:         minInput{Val: "ab"},
			expectedTag:   "min",
			expectedError: "field 'Val' must be at least 5",
		},
		{
			name:          "max",
			input:         maxInput{Val: "abcd"},
			expectedTag:   "max",
			expectedError: "field 'Val' must be at most 3",
		},
		{
			name:          "gte",
			input:         gteInput{Val: 5},
			expectedTag:   "gte",
			expectedError: "field 'Val' must be greater than or equal to 10",
		},
		{
			name:          "lte",
			input:         lteInput{Val: 15},
			expectedTag:   "lte",
			expectedError: "field 'Val' must be less than or equal to 10",
		},
		{
			name:          "url",
			input:         urlInput{Val: "not-a-url"},
			expectedTag:   "url",
			expectedError: "field 'Val' must be a valid URL",
		},
		{
			name:          "uuid",
			input:         uuidInput{Val: "not-a-uuid"},
			expectedTag:   "uuid",
			expectedError: "field 'Val' must be a valid UUID",
		},
		{
			name:          "default handler (alpha)",
			input:         defaultInput{Val: "123"},
			expectedTag:   "alpha",
			expectedError: "field 'Val' failed validation on tag 'alpha'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.input)
			if err == nil {
				t.Fatalf("expected error for %s violation", tt.name)
			}
			var ve ValidationErrors
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationErrors, got %T", err)
			}
			if len(ve) != 1 {
				t.Fatalf("expected 1 error, got %d", len(ve))
			}
			e := ve[0]
			if e.Tag != tt.expectedTag {
				t.Errorf("expected tag %q, got %q", tt.expectedTag, e.Tag)
			}
			if e.Message != tt.expectedError {
				t.Errorf("expected message %q, got %q", tt.expectedError, e.Message)
			}
		})
	}
}

func FuzzSanitizeString(f *testing.F) {
	f.Add("hello world")
	f.Add("<script>alert(1)</script>")
	f.Add("test\x00\x01\x02\x03")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 1024 {
			s = s[:1024]
		}
		result := SanitizeString(s)
		// Result should not contain control characters (except \n, \r, \t).
		for _, r := range result {
			if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
				t.Fatalf("found control character %U in sanitized result", r)
			}
		}
	})
}

func TestGuardConcurrency(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "Validate concurrent execution",
			fn: func() error {
				input := createUserInput{Name: "Bob", Email: "bob@example.com", Age: 40}
				return Validate(input)
			},
		},
		{
			name: "Validate invalid concurrent execution",
			fn: func() error {
				input := createUserInput{Name: "", Email: "invalid", Age: -5}
				err := Validate(input)
				if err == nil {
					return errors.New("expected error but got nil")
				}
				return nil
			},
		},
		{
			name: "SanitizeString concurrent execution",
			fn: func() error {
				got := SanitizeString("  hello\x00world\n  ")
				if got != "helloworld" {
					return fmt.Errorf("unexpected string: %q", got)
				}
				return nil
			},
		},
		{
			name: "StripHTML concurrent execution",
			fn: func() error {
				got := StripHTML("<div><script>alert(1)</script><p>text</p></div>")
				if got != "text" {
					return fmt.Errorf("unexpected string: %q", got)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const numGoroutines = 100
			errCh := make(chan error, numGoroutines)
			for i := 0; i < numGoroutines; i++ {
				go func() {
					errCh <- tt.fn()
				}()
			}
			for i := 0; i < numGoroutines; i++ {
				if err := <-errCh; err != nil {
					t.Errorf("concurrent test failed: %v", err)
				}
			}
		})
	}
}
