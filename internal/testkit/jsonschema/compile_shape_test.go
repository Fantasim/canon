package jsonschema_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/jsonschema"
)

// One case per keyword whose value has a shape the keyword itself does not accept: none of
// these compile, and none is silently accepted (compile_scalar.go, compile_collection.go,
// compile_compose.go).
func TestCompileWrongShapes(t *testing.T) {
	cases := []struct{ name, schema string }{
		{"enum not array", `{"enum": "a"}`},
		{"required not array", `{"required": "a"}`},
		{"properties not object", `{"properties": []}`},
		{"allOf not array", `{"allOf": {}}`},
		{"oneOf not array", `{"oneOf": {}}`},
		{"oneOf empty", `{"oneOf": []}`},
		{"allOf empty", `{"allOf": []}`},
		{"pattern not string", `{"pattern": 5}`},
		{"uniqueItems not bool", `{"uniqueItems": "yes"}`},
		{"dependentRequired not object", `{"dependentRequired": []}`},
		{"type empty array", `{"type": []}`},
		{"type not string or array", `{"type": 5}`},
		{"title not string", `{"title": 5}`},
		{"description not string", `{"description": 5}`},
		{"$id not string", `{"$id": 5}`},
		{"type duplicate", `{"type": ["string", "string"]}`},
		{"required duplicate", `{"required": ["a", "a"]}`},
		{"minItems past int64", `{"minItems": 18446744073709551617}`},
		{"minLength huge exponent", `{"minLength": 1e400}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := jsonschema.Compile([]byte(tc.schema)); err == nil {
				t.Errorf("Compile(%s) = nil error, want one", tc.schema)
			}
		})
	}
}
