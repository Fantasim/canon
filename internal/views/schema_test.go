package views_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
	"github.com/fantasim/canonlang/internal/testkit/jsonschema"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
)

const schemaPath = "../../spec/viewmodel.schema.json"

// fragment compiles the view-model schema with its root replaced by root, which names one of
// its $defs: the validator checks a section before the whole document exists.
func fragment(t *testing.T, root map[string]any) *jsonschema.Schema {
	t.Helper()
	b, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	var full map[string]any
	if err := json.Unmarshal(b, &full); err != nil {
		t.Fatal(err)
	}
	root["$schema"], root["$defs"] = full["$schema"], full["$defs"]
	src, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := jsonschema.Compile(src)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// validate reports every violation of v against s.
func validate(t *testing.T, s *jsonschema.Schema, what string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	for _, e := range s.Validate(b) {
		t.Errorf("%s: %v", what, e)
	}
}

// VIEWMODEL.md V2, J1, J5: the whole model of every example package, as gen/view writes it,
// validates against the schema, and building it twice gives the same bytes; a table control
// without its columns (T1) does not validate.
func TestExampleModelsValidate(t *testing.T) {
	s := wholeSchema(t)
	x := examples(t)
	for _, p := range x.a.Program().Packages {
		out := written(t, x.model(t, p.Path))
		for _, e := range s.Validate(out) {
			t.Errorf("%s: %v", p.Path, e)
		}
		if again := written(t, x.model(t, p.Path)); !bytes.Equal(out, again) {
			t.Errorf("%s: two builds differ", p.Path)
		}
	}
	bad := x.model(t, pipelinePkg)
	v := bad.Values[potionsValue]
	v.Control.Columns = nil
	bad.Values[potionsValue] = v
	if len(s.Validate(written(t, bad))) == 0 {
		t.Error("the schema takes a table without columns")
	}
}

// wholeSchema is the view-model schema (VIEWMODEL.md V2).
func wholeSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	b, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	s, err := jsonschema.Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// written is m's bytes (J1).
func written(t *testing.T, m *vm.ViewModel) []byte {
	t.Helper()
	b, err := viewgen.Write(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// VIEWMODEL.md V2, 12.3: the `types` of every example package validate against the schema's
// typeDef.
func TestExampleTypesValidate(t *testing.T) {
	s := fragment(t, map[string]any{"type": "object", "additionalProperties": map[string]any{"$ref": "#/$defs/typeDef"}})
	if len(s.Validate([]byte(`{"a.T":{"kind":"record","name":"T"}}`))) == 0 {
		t.Fatal("the typeDef schema takes a record without fields")
	}
	x := examples(t)
	for _, p := range x.a.Program().Packages {
		validate(t, s, p.Path, x.model(t, p.Path).Types)
	}
}

// VIEWMODEL.md V2, 12.5: the control of every field of every example record and case
// validates against the schema's control, but tables, whose columns are VIEWMODEL.md 7.2's.
func TestExampleControlsValidate(t *testing.T) {
	s := fragment(t, map[string]any{"$ref": "#/$defs/control"})
	if len(s.Validate([]byte(`{"kind":"duration"}`))) == 0 {
		t.Fatal("the control schema takes a duration without units")
	}
	x := examples(t)
	r := x.resolver()
	for _, p := range x.a.Program().Packages {
		for _, o := range p.Decls {
			for _, d := range bodies(o.Type()) {
				validateFields(t, s, r, d)
			}
		}
	}
}

// validateFields validates the control of each field of the record or case d, but tables.
func validateFields(t *testing.T, s *jsonschema.Schema, r *control.Resolver, d types.Type) {
	t.Helper()
	for _, f := range encode.FieldsOf(d) {
		if ctl := r.Field(d, f); !holdsTable(ctl) {
			validate(t, s, d.String()+"."+f.Name, ctl)
		}
	}
}

// bodies are the record, or the cases of the variant, t declares.
func bodies(t types.Type) []types.Type {
	if t == nil {
		return nil
	}
	switch x := t.Base().(type) {
	case *types.RecordType:
		return []types.Type{x}
	case *types.VariantType:
		out := make([]types.Type, len(x.Cases))
		for i, c := range x.Cases {
			out[i] = c
		}
		return out
	}
	return nil
}

// holdsTable reports a table control anywhere in c.
func holdsTable(c vm.Control) bool {
	if c.Kind == control.CtlTable {
		return true
	}
	for _, in := range []*vm.Control{c.Element, c.Value, c.KeyControl, c.Fallback} {
		if in != nil && holdsTable(*in) {
			return true
		}
	}
	for _, b := range c.Branches {
		if holdsTable(b) {
			return true
		}
	}
	return false
}
