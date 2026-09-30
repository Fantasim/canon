package canon_test

import (
	"context"
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// starLaw adds to editLaw a package d: a map read past a `*` whose item B holds "fr" beside the
// selection.
var starLaw = map[string]string{
	"d/d.canon": `/// D.
package d

/// Names, in English.
let names: {String: String} = load("etc.json", at: "*.us")
`,
	"d/etc.json": "{\n  \"A\": {\n    \"us\": \"Ay\"\n  },\n  \"B\": {\n    \"us\": \"Bee\",\n    \"fr\": \"Be\"\n  }\n}\n",
}

// API.md W1, W4, W5 (log-2026-09-29 M4 B10): a map read past a `*` whose items hold data beside the
// selection is format, the datum named, for Value, DryRun and Edit alike, its item operations too;
// a Set of an entry stays editable.
func TestEditStarBeside(t *testing.T) {
	p, m := openEdit(t, starLaw)
	const datum = "d/etc.json#/B/fr"
	v, err := p.Value(context.Background(), "d:names")
	if err != nil || v.Editable.Reason != canon.ReasonFormat || v.Editable.File != datum {
		t.Errorf("Value d:names: %+v, %v", v.Editable, err)
	}
	if v, err := p.Value(context.Background(), "d:names[B]"); err != nil || v.Editable.Mode != canon.EditJSON {
		t.Errorf("Value d:names[B]: %+v, %v", v.Editable, err)
	}
	for _, op := range []canon.Op{
		canon.Set("d:names", canon.FromJSON([]byte(`{"A": "Ay"}`))),
		canon.Remove("d:names[A]"),
		canon.Move("d:names[B]", 0),
		canon.AddEntry("d:names", canon.Key("C"), canon.Str("See")),
	} {
		for _, dry := range []bool{true, false} {
			_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{op}, DryRun: dry})
			var ne *canon.NotEditableError
			if !errors.As(err, &ne) || ne.Reason != canon.ReasonFormat || ne.Detail != datum {
				t.Errorf("%+v, DryRun %v: %v", op, dry, err)
			}
		}
	}
	if got := read(t, m, "d/etc.json"); got != starLaw["d/etc.json"] || len(journals(m)) != 0 {
		t.Errorf("a refused edit wrote:\n%s", got)
	}
}

// API.md W1, W4, W5 (log-2026-09-29 M4 B10-r2): an empty map read `at: "*[1]"` takes no entry, since
// no item can hold an element past the first alone: format, the container named, for Value, a
// DryRun and an Edit alike.
func TestEditStarIndex(t *testing.T) {
	p, _ := openEdit(t, map[string]string{
		"d/d.canon":    "/// D.\npackage d\n\n/// Seconds.\nlet seconds: {String: String} = load(\"pairs.json\", at: \"*[1]\")\n",
		"d/pairs.json": "{}\n",
	})
	const container = "d/pairs.json#"
	v, err := p.Value(context.Background(), "d:seconds")
	if err != nil || v.Editable.Reason != canon.ReasonFormat || v.Editable.File != container {
		t.Errorf("Value d:seconds: %+v, %v", v.Editable, err)
	}
	for _, dry := range []bool{true, false} {
		op := canon.AddEntry("d:seconds", canon.Key("a"), canon.Str("w"))
		_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{op}, DryRun: dry})
		var ne *canon.NotEditableError
		if !errors.As(err, &ne) || ne.Reason != canon.ReasonFormat || ne.Detail != container {
			t.Errorf("DryRun %v: %v", dry, err)
		}
	}
}
