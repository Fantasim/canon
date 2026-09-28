package jsonschema_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/jsonschema"
)

// The real schema compiles: VIEWMODEL.md §14 V2.
func TestCompileRealSchema(t *testing.T) {
	b := readViewModelSchema(t)
	if _, err := jsonschema.Compile(b); err != nil {
		t.Fatalf("Compile(viewmodel.schema.json) = %v, want nil", err)
	}
}

// Any keyword outside doc.go's vocabulary is a compile error, so a rule can never be silently
// skipped.
func TestCompileUnknownKeyword(t *testing.T) {
	_, err := jsonschema.Compile([]byte(`{"type": "string", "format": "date"}`))
	if err == nil {
		t.Fatal("Compile with an unknown keyword = nil error, want one")
	}
}

// A remote $ref (not "#/$defs/<name>") is a compile error.
func TestCompileNonLocalRef(t *testing.T) {
	cases := []struct{ name, src string }{
		{"http url", `{"$ref": "http://example.com/schema.json"}`},
		{"root pointer", `{"$ref": "#"}`},
		{"not under defs", `{"$ref": "#/properties/foo"}`},
	}
	for _, tc := range cases {
		if _, err := jsonschema.Compile([]byte(tc.src)); err == nil {
			t.Errorf("%s: Compile = nil error, want one", tc.name)
		}
	}
}

// $ref naming an entry $defs does not have is a compile error.
func TestCompileUndefinedRef(t *testing.T) {
	_, err := jsonschema.Compile([]byte(`{"$ref": "#/$defs/missing", "$defs": {"other": true}}`))
	if err == nil {
		t.Fatal("Compile with an undefined $ref = nil error, want one")
	}
}

// A schema slot that is neither an object nor a boolean fails to compile.
func TestCompileNotASchema(t *testing.T) {
	_, err := jsonschema.Compile([]byte(`{"items": 1}`))
	if err == nil {
		t.Fatal("Compile with items: 1 = nil error, want one")
	}
}

// The schema root itself must be a JSON object.
func TestCompileRootNotObject(t *testing.T) {
	_, err := jsonschema.Compile([]byte(`"not a schema"`))
	if err == nil {
		t.Fatal("Compile of a root string = nil error, want one")
	}
}

// Malformed schema JSON is reported, not panicked on.
func TestCompileBadJSON(t *testing.T) {
	if _, err := jsonschema.Compile([]byte(`{`)); err == nil {
		t.Fatal("Compile of truncated JSON = nil error, want one")
	}
}
