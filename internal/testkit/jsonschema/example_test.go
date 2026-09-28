package jsonschema_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/jsonschema"
)

// A schema compiles once, then validates any number of documents.
func ExampleCompile() {
	schema, err := jsonschema.Compile([]byte(`{
		"type": "object",
		"properties": {"name": {"type": "string", "minLength": 1}},
		"required": ["name"],
		"additionalProperties": false
	}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	errs := schema.Validate([]byte(`{"name": ""}`))
	fmt.Println(errs[0])
	// Output: /name: /properties/name/minLength: want at least 1 code points, got 0
}
