package guard

import (
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
)

type createUserInput struct {
	Name  string `validate:"required,min=2,max=100"`
	Email string `validate:"required,email"`
	Age   int    `validate:"gte=0,lte=150"`
}

func TestValidateSuccess(t *testing.T) {
	input := createUserInput{
		Name:  "Alice",
		Email: "alice@example.com",
		Age:   30,
	}
	if err := Validate(input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateFailure(t *testing.T) {
	input := createUserInput{
		Name:  "",
		Email: "not-an-email",
		Age:   -1,
	}
	err := Validate(input)
	if err == nil {
		t.Fatal("expected validation error")
	}

	var ve ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
	}
	if len(ve) != 3 {
		t.Fatalf("expected 3 errors, got %d: %v", len(ve), ve)
	}
}

func TestValidateRequiredOnly(t *testing.T) {
	input := createUserInput{
		Name:  "",
		Email: "valid@example.com",
		Age:   25,
	}
	err := Validate(input)
	if err == nil {
		t.Fatal("expected validation error for missing name")
	}
	var ve ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	found := false
	for _, e := range ve {
		if e.Field == "Name" && e.Tag == "required" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected required error for Name")
	}
}

func TestValidateNonStructInput(t *testing.T) {
	// Passing a non-struct value triggers the non-validator.ValidationErrors branch.
	err := Validate("not a struct")
	if err == nil {
		t.Fatal("expected error for non-struct input")
	}
	// The underlying error from validator is NOT validator.ValidationErrors,
	// so it should be returned as-is.
	var ve ValidationErrors
	if errors.As(err, &ve) {
		t.Fatal("should not be ValidationErrors for non-struct input")
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

func TestFormatValidationErrorAllTags(t *testing.T) {
	// Test all tag formats via actual validation failures.

	// min tag — without required, so min is the failing tag.
	type minInput struct {
		Value string `validate:"min=5"`
	}
	err := Validate(minInput{Value: "ab"})
	if err == nil {
		t.Fatal("expected error for min violation")
	}
	var ve ValidationErrors
	if errors.As(err, &ve) {
		found := false
		for _, e := range ve {
			if e.Tag == "min" {
				found = true
			}
		}
		if !found {
			t.Fatal("expected min validation error")
		}
	}

	type urlInput struct {
		URL string `validate:"required,url"`
	}
	err = Validate(urlInput{URL: "not a url"})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if errors.As(err, &ve) {
		found := false
		for _, e := range ve {
			if e.Tag == "url" {
				found = true
			}
		}
		if !found {
			t.Fatal("expected url validation error")
		}
	}

	type uuidInput struct {
		ID string `validate:"required,uuid"`
	}
	err = Validate(uuidInput{ID: "not-a-uuid"})
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}
	if errors.As(err, &ve) {
		found := false
		for _, e := range ve {
			if e.Tag == "uuid" {
				found = true
			}
		}
		if !found {
			t.Fatal("expected uuid validation error")
		}
	}

	type maxInput struct {
		Value string `validate:"max=3"`
	}
	err = Validate(maxInput{Value: "toolong"})
	if err == nil {
		t.Fatal("expected error for max length exceeded")
	}

	type lteInput struct {
		Value int `validate:"lte=10"`
	}
	err = Validate(lteInput{Value: 20})
	if err == nil {
		t.Fatal("expected error for lte violation")
	}

	// Default tag (one not explicitly listed in the switch).
	type alphaInput struct {
		Value string `validate:"alpha"`
	}
	err = Validate(alphaInput{Value: "123"})
	if err == nil {
		t.Fatal("expected error for alpha violation")
	}
	if errors.As(err, &ve) {
		found := false
		for _, e := range ve {
			if e.Tag == "alpha" {
				found = true
			}
		}
		if !found {
			t.Fatal("expected alpha validation error (default handler)")
		}
	}
}

func FuzzSanitizeString(f *testing.F) {
	f.Add("hello world")
	f.Add("<script>alert(1)</script>")
	f.Add("test\x00\x01\x02\x03")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		result := SanitizeString(s)
		// Result should not contain control characters (except \n, \r, \t).
		for _, r := range result {
			if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
				t.Fatalf("found control character %U in sanitized result", r)
			}
		}
	})
}
