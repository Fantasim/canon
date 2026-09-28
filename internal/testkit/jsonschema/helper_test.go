package jsonschema_test

import (
	"os"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/jsonschema"
)

// Repository-relative to this package's directory (IMPLEMENTATION-PLAN.md's layout).
const (
	viewModelSchemaPath    = "../../../spec/viewmodel.schema.json"
	pipelinePotionViewPath = "../../../examples/pipeline/expected/potion.view.json"
)

func readViewModelSchema(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(viewModelSchemaPath)
	if err != nil {
		t.Fatalf("read %s: %v", viewModelSchemaPath, err)
	}
	return b
}

func readPipelinePotionView(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(pipelinePotionViewPath)
	if err != nil {
		t.Fatalf("read %s: %v", pipelinePotionViewPath, err)
	}
	return b
}

// mustCompile compiles src or fails the test; every negative-case test compiles a small inline
// schema this way.
func mustCompile(t *testing.T, src string) *jsonschema.Schema {
	t.Helper()
	schema, err := jsonschema.Compile([]byte(src))
	if err != nil {
		t.Fatalf("Compile(%s) = %v, want nil", src, err)
	}
	return schema
}
