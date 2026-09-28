package views_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/views"
	"github.com/fantasim/canonlang/internal/views/control"
)

const (
	pipelineGolden = "../../examples/pipeline/expected/potion.view.json"
	pipelinePkg    = "pipeline"
	potionType     = "pipeline.Potion"
	potionName     = "Potion"
	potionsValue   = "pipeline:potions"
)

// golden is the pipeline golden, hand-drafted (VIEWMODEL.md §15.1, DOCTRINE §4's exception).
func golden(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(pipelineGolden)
	if err != nil {
		t.Fatal(err)
	}
	return decode(t, b).(map[string]any)
}

// member is m[keys[0]][keys[1]]…, nil when one is missing.
func member(m any, keys ...string) any {
	for _, k := range keys {
		obj, ok := m.(map[string]any)
		if !ok {
			return nil
		}
		m = obj[k]
	}
	return m
}

// VIEWMODEL.md 15.1, 12.3: the pipeline's `types` equal the golden's: `heal` an int of 1 to
// 100 000, `cooldown` a duration of 0 to 600 000 ms with wire unit ms, `stack` defaulted to 99
// and not required, the doc comments as help keys.
func TestPipelineTypes(t *testing.T) {
	m := examples(t).model(t, pipelinePkg)
	got, want := canonical(t, m.Types), member(golden(t), "types")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("types:\n got %s\nwant %s", text(got), text(want))
	}
	if m.Schema != views.SchemaVersion || m.Package != pipelinePkg || m.Language != language {
		t.Errorf("envelope = %q %q %q", m.Schema, m.Package, m.Language)
	}
}

// VIEWMODEL.md 15.1, C11, C12, C13, X12, C45: each Potion field's control is the golden's: `id`
// and `name` inputs with their patterns, `heal` a number in hp without a stepper, `cooldown` a
// duration offering ms, s and m, `stack` a number.
func TestPipelineFieldControls(t *testing.T) {
	x := examples(t)
	r := x.resolver()
	potion := x.named(t, pipelinePkg, potionName)
	want := golden(t)
	for _, name := range []string{"id", "name", "heal", "cooldown", "stack"} {
		got := canonical(t, r.Field(potion, field(t, potion, name)))
		if w := member(want, "views", potionType, "fields", name, "control"); !reflect.DeepEqual(got, w) {
			t.Errorf("%s control:\n got %s\nwant %s", name, text(got), text(w))
		}
	}
}

// VIEWMODEL.md C26: `potions`, a keyed list of Potion, is a table of Potion; its key, columns
// and search are T1's.
func TestPipelineValueControl(t *testing.T) {
	x := examples(t)
	got := x.resolver().Value(nil, x.letType(t, pipelinePkg, "potions"))
	w := member(golden(t), "values", potionsValue, "control")
	if got.Kind != control.CtlTable || got.Of != member(w, "of") {
		t.Errorf("potions control = %s, want kind table and the of of %s", text(got), text(w))
	}
}
