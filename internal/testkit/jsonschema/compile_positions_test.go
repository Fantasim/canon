package jsonschema_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/jsonschema"
)

// An unknown keyword fails Compile at every position a subschema can occur, not just the root.
func TestCompileUnknownKeywordPositions(t *testing.T) {
	cases := []struct{ name, schema string }{
		{"root", `{"bogus": true}`},
		{"$defs", `{"$defs": {"a": {"bogus": true}}}`},
		{"properties/x", `{"properties": {"x": {"bogus": true}}}`},
		{"items", `{"items": {"bogus": true}}`},
		{"additionalProperties", `{"additionalProperties": {"bogus": true}}`},
		{"propertyNames", `{"propertyNames": {"bogus": true}}`},
		{"if", `{"if": {"bogus": true}, "then": {}}`},
		{"then", `{"if": {}, "then": {"bogus": true}}`},
		{"oneOf/1", `{"oneOf": [{"type": "string"}, {"bogus": true}]}`},
		{"allOf/1", `{"allOf": [{"type": "string"}, {"bogus": true}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := jsonschema.Compile([]byte(tc.schema)); err == nil {
				t.Errorf("Compile(%s) = nil error, want one", tc.schema)
			}
		})
	}
}
