package jsonschema_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/jsonschema"
)

// Compile and Validate both decode through internal/jsonsrc.Parse, stricter than
// encoding/json alone: trailing content, a duplicate key and invalid UTF-8 all fail.
func TestDecodeStrictSchema(t *testing.T) {
	cases := []struct{ name, src string }{
		{"trailing object", `{} {}`},
		{"trailing garbage", `{} garbage`},
		{"duplicate key", `{"type": "string", "type": "number"}`},
		{"invalid UTF-8", "{\"type\": \"\xff\"}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := jsonschema.Compile([]byte(tc.src)); err == nil {
				t.Errorf("Compile(%s) = nil error, want one", tc.src)
			}
		})
	}
}

// Validate's own decode is exactly as strict as Compile's.
func TestDecodeStrictInstance(t *testing.T) {
	schema := mustCompile(t, `{}`)
	cases := []struct{ name, src string }{
		{"trailing object", `{} {}`},
		{"trailing garbage", `{} garbage`},
		{"duplicate key", `{"a": 1, "a": 2}`},
		{"invalid UTF-8", "{\"a\": \"\xff\"}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := schema.Validate([]byte(tc.src))
			if len(errs) == 0 {
				t.Errorf("Validate(%s) = no errors, want one", tc.src)
			}
		})
	}
}
