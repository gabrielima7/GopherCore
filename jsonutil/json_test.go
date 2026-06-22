package jsonutil

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

type testStruct struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email,omitempty"`
}

func TestMarshal(t *testing.T) {
	s := testStruct{Name: "Alice", Age: 30}
	data, err := Marshal(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `{"name":"Alice","age":30}`
	if string(data) != expected {
		t.Fatalf("expected %s, got %s", expected, string(data))
	}
}

func TestMarshalIndent(t *testing.T) {
	s := testStruct{Name: "Bob", Age: 25}
	data, err := MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), "\n") {
		t.Fatal("expected indented output with newlines")
	}
}

func TestUnmarshal(t *testing.T) {
	raw := `{"name":"Charlie","age":40,"email":"charlie@example.com"}`
	var s testStruct
	if err := Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "Charlie" || s.Age != 40 || s.Email != "charlie@example.com" {
		t.Fatalf("unexpected result: %+v", s)
	}
}

func TestRoundtrip(t *testing.T) {
	original := testStruct{Name: "Dana", Age: 35, Email: "dana@test.com"}
	data, err := Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded testStruct
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if original != decoded {
		t.Fatalf("roundtrip mismatch: %+v != %+v", original, decoded)
	}
}

func TestUnmarshalNil(t *testing.T) {
	var s testStruct
	err := Unmarshal(nil, &s)
	if err == nil {
		t.Fatal("expected error for nil input")
	}
}

func TestUnmarshalInvalidJSON(t *testing.T) {
	var s testStruct
	err := Unmarshal([]byte("{invalid"), &s)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestEncoder(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(testStruct{Name: "Eve", Age: 28}); err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if !strings.Contains(buf.String(), "Eve") {
		t.Fatal("expected output to contain 'Eve'")
	}
}

func TestDecoder(t *testing.T) {
	input := `{"name":"Frank","age":50}`
	dec := NewDecoder(strings.NewReader(input))
	var s testStruct
	if err := dec.Decode(&s); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if s.Name != "Frank" || s.Age != 50 {
		t.Fatalf("unexpected result: %+v", s)
	}
}

func TestValid_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{"valid object", []byte(`{"key":"value"}`), true},
		{"valid array", []byte(`[1, 2, 3]`), true},
		{"valid string", []byte(`"hello"`), true},
		{"valid number", []byte(`123`), true},
		{"valid boolean", []byte(`true`), true},
		{"valid null", []byte(`null`), true},
		{"missing quotes", []byte(`{key:"value"}`), false},
		{"truncated array", []byte(`[1, 2, `), false},
		{"trailing comma object", []byte(`{"key":"value",}`), false},
		{"trailing comma array", []byte(`[1, 2,]`), false},
		{"empty byte slice", []byte(""), false},
		{"nil byte slice", nil, false},
		{"invalid char", []byte(`{invalid`), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Valid(tt.input); got != tt.expected {
				t.Errorf("Valid(%q) = %v; expected %v", tt.input, got, tt.expected)
			}
		})
	}
}

type errorWriter struct{}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("simulated write error")
}

func TestEncoder_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		data         any
		writer       *bytes.Buffer
		useErrWriter bool
		expectErr    bool
	}{
		{"valid encode", testStruct{Name: "Eve", Age: 28}, &bytes.Buffer{}, false, false},
		{"unmarshalable channel", make(chan int), &bytes.Buffer{}, false, true},
		{"writer error", testStruct{Name: "Eve", Age: 28}, nil, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.useErrWriter {
				enc := NewEncoder(&errorWriter{})
				err = enc.Encode(tt.data)
			} else {
				enc := NewEncoder(tt.writer)
				err = enc.Encode(tt.data)
			}
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error: %v, got: %v", tt.expectErr, err)
			}
		})
	}
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

func TestDecoder_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		useErrReader bool
		expectErr    bool
	}{
		{"valid decode", `{"name":"Frank","age":50}`, false, false},
		{"invalid json", `{"name":"Frank","age":}`, false, true},
		{"reader error", ``, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.useErrReader {
				dec := NewDecoder(&errorReader{})
				var s testStruct
				err = dec.Decode(&s)
			} else {
				dec := NewDecoder(strings.NewReader(tt.input))
				var s testStruct
				err = dec.Decode(&s)
			}
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error: %v, got: %v", tt.expectErr, err)
			}
		})
	}
}

func TestMarshalIndent_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		data      any
		expectErr bool
	}{
		{"valid struct", testStruct{Name: "Bob", Age: 25}, false},
		{"unmarshalable channel", make(chan int), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := MarshalIndent(tt.data, "", "  ")
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error: %v, got: %v", tt.expectErr, err)
			}
		})
	}
}

func TestConcurrency_ThreadSafety(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	data := testStruct{Name: "Alice", Age: 30}
	marshaledData := []byte(`{"name":"Alice","age":30}`)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = Marshal(data)
		}()

		go func() {
			defer wg.Done()
			var s testStruct
			_ = Unmarshal(marshaledData, &s)
		}()

		go func() {
			defer wg.Done()
			_ = Valid(marshaledData)
		}()
	}

	wg.Wait()
}

func TestMarshalNestedStruct(t *testing.T) {
	type inner struct {
		ID int `json:"id"`
	}
	type outer struct {
		Name  string `json:"name"`
		Inner inner  `json:"inner"`
	}
	o := outer{Name: "test", Inner: inner{ID: 42}}
	data, err := Marshal(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), `"id":42`) {
		t.Fatalf("expected nested struct in output: %s", string(data))
	}
}

func FuzzMarshalUnmarshal(f *testing.F) {
	f.Add("Alice", 30)
	f.Add("", 0)
	f.Add("José María", -1)
	f.Fuzz(func(t *testing.T, name string, age int) {
		if !utf8.ValidString(name) {
			return
		}
		original := testStruct{Name: name, Age: age}
		data, err := Marshal(original)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var decoded testStruct
		if err := Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if original.Name != decoded.Name || original.Age != decoded.Age {
			t.Fatalf("roundtrip mismatch: %+v != %+v", original, decoded)
		}
	})
}
