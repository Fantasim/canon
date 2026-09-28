package jsonschema_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/jsonschema"
)

// TestPipelinePotionView gates the pipeline golden on the real schema (VIEWMODEL.md's V2).
func TestPipelinePotionView(t *testing.T) {
	schema, err := jsonschema.Compile(readViewModelSchema(t))
	if err != nil {
		t.Fatalf("Compile(viewmodel.schema.json) = %v, want nil", err)
	}
	for _, e := range schema.Validate(readPipelinePotionView(t)) {
		t.Errorf("potion.view.json: %s", e)
	}
}
