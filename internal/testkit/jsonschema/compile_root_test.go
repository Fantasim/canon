package jsonschema_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/jsonschema"
)

// $defs, $id and $schema are refused anywhere but the root: a nested $defs can never shadow
// the root's own.
func TestCompileRootOnlyKeywordsElsewhere(t *testing.T) {
	cases := []struct{ name, schema string }{
		{"$defs nested", `{"properties": {"a": {"$defs": {"x": true}}}}`},
		{"$id nested", `{"properties": {"a": {"$id": "urn:x"}}}`},
		{"$schema nested", `{"properties": {"a": {"$schema": "urn:x"}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := jsonschema.Compile([]byte(tc.schema)); err == nil {
				t.Errorf("Compile(%s) = nil error, want one", tc.schema)
			}
		})
	}
}

// A nested $defs cannot shadow a root def of the same name, since it never compiles at all.
func TestCompileNestedDefsCannotShadowRoot(t *testing.T) {
	schema := `{
		"$ref": "#/$defs/a", "$defs": {"a": {"minimum": 0}},
		"properties": {"b": {"$defs": {"a": {"minimum": 100}}}}
	}`
	if _, err := jsonschema.Compile([]byte(schema)); err == nil {
		t.Fatal("Compile with a nested $defs = nil error, want one")
	}
}

// The root's $schema, when present, must name draft 2020-12.
func TestCompileSchemaURI(t *testing.T) {
	if _, err := jsonschema.Compile([]byte(`{"$schema": "https://json-schema.org/draft-07/schema"}`)); err == nil {
		t.Error("Compile with a draft-07 $schema = nil error, want one")
	}
	if _, err := jsonschema.Compile([]byte(`{"$schema": "https://json-schema.org/draft/2020-12/schema"}`)); err != nil {
		t.Errorf("Compile with the 2020-12 $schema = %v, want nil", err)
	}
}
