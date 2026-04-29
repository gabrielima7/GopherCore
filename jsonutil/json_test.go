package jsonutil

import (
	"bytes"
	"strings"
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

func TestValid(t *testing.T) {
	if !Valid([]byte(`{"key":"value"}`)) {
		t.Fatal("expected valid JSON")
	}
	if Valid([]byte("{invalid")) {
		t.Fatal("expected invalid JSON")
	}
	if Valid(nil) {
		t.Fatal("expected nil to be invalid")
	}
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
