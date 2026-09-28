package jsonschema_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/jsonschema"
)

// A $ref/allOf/oneOf/if/then chain that never reaches a different part of the instance is a
// compile error, not a stack overflow at Validate time.
func TestCompileRefCycle(t *testing.T) {
	cases := []struct{ name, schema string }{
		{"self $ref", `{"$ref": "#/$defs/a", "$defs": {"a": {"$ref": "#/$defs/a"}}}`},
		{"mutual $ref", `{
			"$ref": "#/$defs/a",
			"$defs": {"a": {"$ref": "#/$defs/b"}, "b": {"$ref": "#/$defs/a"}}
		}`},
		{"through allOf", `{
			"$ref": "#/$defs/a",
			"$defs": {"a": {"allOf": [{"$ref": "#/$defs/a"}]}}
		}`},
		{"through if/then", `{
			"$ref": "#/$defs/a",
			"$defs": {"a": {"if": true, "then": {"$ref": "#/$defs/a"}}}
		}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := jsonschema.Compile([]byte(tc.schema)); err == nil {
				t.Errorf("Compile(%s) = nil error, want one", tc.schema)
			}
		})
	}
}

// Recursion through items, properties, additionalProperties or propertyNames is fine: each
// reaches a different part of the instance, so it is bounded by the instance's own depth.
func TestCompileRecursiveDefThroughItemsIsFine(t *testing.T) {
	schema := `{
		"$ref": "#/$defs/tree",
		"$defs": {"tree": {"type": "array", "items": {"$ref": "#/$defs/tree"}}}
	}`
	s, err := jsonschema.Compile([]byte(schema))
	if err != nil {
		t.Fatalf("Compile(%s) = %v, want nil", schema, err)
	}
	if errs := s.Validate([]byte(`[[], [[]]]`)); len(errs) != 0 {
		t.Errorf("Validate([[], [[]]]) = %v, want none", errs)
	}
	if errs := s.Validate([]byte(`[1]`)); len(errs) == 0 {
		t.Error("Validate([1]) = no errors, want a type violation")
	}
}
