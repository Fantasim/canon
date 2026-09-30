package edit_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// heldRoot is depRoot with a second objective and a step (its move inline) holding, in their Never
// branch, the symbol the decoder reads from "anywhere" (DECISIONS 175).
var heldRoot = strings.NewReplacer(`      "target": "GREEN"
    }
  ],`, `      "target": "GREEN"
    },
    {
      "def": "tour",
      "target": "anywhere"
    }
  ],`, `  "ctag": {
    "pick": "word"
  }
}`, `  "ctag": {
    "pick": "word"
  },
  "steps": [
    {
      "name": "s",
      "kind": "go",
      "def": "tour",
      "target": "anywhere"
    }
  ]
}`).Replace(depRoot)

// API.md V1 (log-2026-09-29 M4 B7): a name the edit introduces into a Never branch is refused,
// in a record, an added record or the field alone, as a Set of the field alone refuses it.
func TestSetJSONDependentSymbolNever(t *testing.T) {
	s := open(t, depFSWith(heldRoot), nil, "", "d")
	for _, op := range []edit.Operation{
		{Kind: edit.OpAdd, Path: "root.objs", Value: edit.Source("{ def: tour, target: anywhere }")},
		{Kind: edit.OpSet, Path: "root.objs[0]", Value: edit.Source("{ def: tour, target: anywhere }")},
		{Kind: edit.OpSet, Path: "root.objs[1]", Value: edit.Source("{ def: tour, target: elsewhere }")},
		{Kind: edit.OpSet, Path: "root.objs[1].target", Value: edit.Source("elsewhere")},
	} {
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
		var ve *edit.ValueError
		if !errors.As(err, &ve) || ve.Expected != "Never" {
			t.Errorf("%s: %v, want a *ValueError expecting Never", op.Path, err)
		}
	}
}

// DECISIONS 175, API.md E15 (log-2026-09-29 M4 B7): a symbol the record already held in its Never
// branch is written back as the string it was read from, by a Set of its record keeping it and
// as the value a cascade drops.
func TestSetJSONDependentSymbolHeld(t *testing.T) {
	fsys := depFSWith(heldRoot)
	applyDepOn(t, fsys, symCase{"record", edit.Operation{Kind: edit.OpSet, Path: "root.objs[1]", Value: edit.Source(`{ def: tour, target: anywhere, note: "x" }`)},
		"d/root.json", []string{`"target": "anywhere"`, `"note": "x"`}})
	plan := applyDepOn(t, fsys, symCase{"cascade", edit.Operation{Kind: edit.OpSet, Path: "root.objs[1].def", Value: edit.Source("art")}, "d/root.json", []string{`"def": "art"`}})
	if len(plan.Dropped) != 1 || string(plan.Dropped[0].Value) != `"anywhere"` {
		t.Errorf("E15: dropped %+v, want the held symbol as \"anywhere\"", plan.Dropped)
	}
}

// API.md E22, E23, M6 (log-2026-09-29 M4 B7-r, B7-r2): the Undo of a Set replacing a value that
// held a decoded symbol carries it as FromJSON of its wire, re-encoded, and gives the bytes back
// exactly: E15 leaves the restored symbol, discriminant set back or not, inline too.
func TestUndoJSONDependentSymbolRestored(t *testing.T) {
	for _, c := range []struct {
		name, path, set, wire string
	}{
		{"discriminant kept", "root.objs[1]", `{ def: tour, note: "x" }`, heldWire},
		{"discriminant set back", "root.objs[1]", "{ def: art, target: red }", heldWire},
		{"inline variant", "root.steps[0].act", "go { def: art, target: red }", `{"kind":"go","def":"tour","target":"anywhere"}`},
	} {
		fsys := depFSWith(heldRoot)
		plan := applyDepOn(t, fsys, symCase{c.name, edit.Operation{Kind: edit.OpSet, Path: c.path, Value: edit.Source(c.set)}, "d/root.json", nil})
		if len(plan.Undo) != 1 || !reflect.DeepEqual(plan.Undo[0].Value, edit.FromJSON(c.wire)) {
			t.Errorf("%s: E23: undo %+v, want Set(%s)", c.name, plan.Undo, c.wire)
			continue
		}
		back := open(t, written(fsys, plan), nil, "", "d")
		undo, err := edit.Apply(context.Background(), back.env, back.snap, edit.Request{Ops: plan.Undo})
		if err != nil {
			t.Errorf("%s: E22: the Undo: %v", c.name, err)
			continue
		}
		for _, ch := range undo.Changes {
			checkWritten(t, undo, ch)
		}
		if got := string(written(fsys, undo)["law/d/root.json"].Data); got != heldRoot || len(undo.Dropped) != 0 {
			t.Errorf("%s: E22: the Undo does not give root.json back (dropped %+v):\n%s", c.name, undo.Dropped, got)
		}
	}
}

// heldWire is heldRoot's second objective, compacted.
const heldWire = `{"def":"tour","target":"anywhere"}`

// DEP-02, TYP-15 (log-2026-09-29 M4 B7): a discriminant a record leaves to its default selects the
// branch as the decoder reads it: a name its branch holds is written, one it lacks is refused.
func TestSetJSONDependentSymbolDefault(t *testing.T) {
	plan := applyDep(t, symCase{"paint by default", edit.Operation{Kind: edit.OpSet, Path: "root.tag", Value: edit.Source("{ pick: green }")}, "d/root.json", []string{`"pick": "GREEN"`}})
	if back := open(t, written(depFS(), plan), nil, "", "d"); back.a.Result().Summary.Errors > 0 {
		t.Errorf("the written source does not decode: %v", back.a.Result().List)
	}
	s := open(t, depFS(), nil, "", "d")
	_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{{Kind: edit.OpSet, Path: "root.ctag", Value: edit.Source("{ pick: word }")}}})
	var ve *edit.ValueError
	if !errors.As(err, &ve) || ve.Expected != "String" || ve.Got != "word" {
		t.Errorf("a String branch from a default: %v, want a *ValueError", err)
	}
}

// DEP-02, API.md V1, M7, M8 (log-2026-09-29 M4 B7): a bare name keys a map of a dependent key
// type; a JSON source writes what it names, a .canon source keeps the name.
func TestSetDependentKey(t *testing.T) {
	src := func(s string) edit.Lit { return edit.Source(s) }
	for _, c := range []symCase{
		{"JSON map", edit.Operation{Kind: edit.OpSet, Path: "root.tag.byPick", Value: src("{ green: 2 }")}, "d/root.json", []string{`"GREEN": 2`}},
		{"JSON entry", edit.Operation{Kind: edit.OpAddEntry, Path: "root.tag.byPick", Key: edit.Key("green"), Value: edit.Int(3)}, "d/root.json", []string{`"GREEN": 3`}},
		{"Canon map", edit.Operation{Kind: edit.OpSet, Path: "canonTag.byPick", Value: src("{ red: 2 }")}, "d/d.canon", []string{"byPick: { red: 2 }"}},
	} {
		plan := applyDep(t, c)
		if back := open(t, written(depFS(), plan), nil, "", "d"); back.a.Result().Summary.Errors > 0 {
			t.Errorf("%s: the written source does not check: %v", c.name, back.a.Result().List)
		}
	}
}
