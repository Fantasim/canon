package edit_test

import (
	"errors"
	"path/filepath"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/edit"
)

// mapLinesP is a package whose maps the layers amend by key: a map a field default states, one
// a spread states, one a load states, one only layers fill.
const mapLinesP = `/// P.
package p

/// A configuration.
record Conf {
  /// Weights by name.
  weights: {String: Int} = { "a": 1 }
}

/// By its field's default.
let byDefault: Conf = {}
/// A seed.
let seed: Conf = { weights: { "a": 1 } }
/// Through a spread.
let spread: Conf = { ...seed }
/// Loaded.
let loaded: Conf = load("c.json")
/// Filled by layers.
let stacked: Conf = {}

/// Two counts.
record Pair {
  /// One.
  n: Int = 0
  /// Two.
  m: Int = 0
}

/// Amended by two layers, a field each.
let src: Pair = {}
`

// mapLines is mapLinesP with its layers: dev re-values a key each map states and the one lx adds,
// di (never active) re-values one and adds another.
var mapLines = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/p/p.canon":     file(mapLinesP),
	"law/p/c.json":      file("{\"weights\": {\"a\": 1}}\n"),
	"law/p/dev.layer.canon": file("package p\nlayer dev\n\namend byDefault {\n  weights[\"a\"]: 5\n}\n\namend spread {\n  weights[\"a\"]: 5\n}\n\n" +
		"amend loaded {\n  weights[\"a\"]: 5\n}\n\namend stacked {\n  weights[\"zz\"]: 7\n}\n\namend src {\n  m: 2\n}\n"),
	"law/p/lx.layer.canon": file("package p\nlayer lx\n\namend stacked {\n  weights[\"zz\"]: 3\n}\n\namend src {\n  n: 1\n}\n"),
	"law/p/di.layer.canon": file("package p\nlayer di\n\namend byDefault {\n  weights[\"a\"]: 5\n  weights[\"zz\"]: 9\n}\n"),
}

// API.md E22, W11a, 7.2 (log-2026-11-01 EDIT-OWED review, round-5 review): a Set replacing the edit
// layer's key lines of a map a default, a spread or a load states applies and its Undo restores
// every value; where lx amends the key, the Set and the Reset of that line are `layered` by lx.
func TestE22MapLineUndoSources(t *testing.T) {
	set := func(root string) edit.Operation {
		return edit.Operation{Kind: edit.OpSet, Path: "p:" + root + ".weights", Value: edit.Source(`{ "b": 2 }`)}
	}
	dev := goldenCase{fsys: mapLines, pkgs: []string{"p"}, layers: []string{"dev"}, editLayer: "dev"}
	for _, root := range []string{"byDefault", "spread", "loaded"} {
		applyAndUndo(t, dev, set(root))
	}
	stacked := goldenCase{fsys: mapLines, pkgs: []string{"p"}, layers: []string{"lx", "dev"}, editLayer: "dev"}
	checkLayeredBy(t, stacked, set("stacked"), "lx") // its Undo puts back lx's key by a Set no true restore exempts
	checkLayeredBy(t, stacked, edit.Operation{Kind: edit.OpReset, Path: `p:stacked.weights["zz"]`}, "lx")
}

// API.md 7.2 layered, E22 (log-2026-11-01 EDIT-OWED re-review 3, round-5 review): lx amending
// only a descendant, a Set of the map is `layered`: its Undo's Set of the key is no true restore
// (round-6 review); a user's Set of the key lx amends is `layered` too.
func TestLayeredAboveEditLayer(t *testing.T) {
	stack := goldenCase{fsys: mapLines, pkgs: []string{"p"}, layers: []string{"dev", "lx"}, editLayer: "dev"}
	checkLayeredBy(t, stack, edit.Operation{Kind: edit.OpSet, Path: "p:stacked.weights", Value: edit.Source(`{ "b": 2 }`)}, "lx")
	checkLayeredBy(t, stack, edit.Operation{Kind: edit.OpSet, Path: `p:stacked.weights["zz"]`, Value: edit.Int(4)}, "lx")
}

// API.md 7.2 layered (log-2026-11-01 EDIT-OWED round-5 review): a Set of a path another active
// layer amends is `layered`, naming that layer, whatever the stack order, the edit layer active or not.
func TestLayeredWhateverTheStack(t *testing.T) {
	for _, layers := range [][]string{{"lx", "dev"}, {"dev", "lx"}, {"lx"}} {
		c := goldenCase{fsys: mapLines, pkgs: []string{"p"}, layers: layers, editLayer: "dev"}
		checkLayeredBy(t, c, edit.Operation{Kind: edit.OpSet, Path: "p:src.n", Value: edit.Int(9)}, "lx")
	}
}

// checkLayeredBy fails t unless Editable and Apply refuse op in c as `layered` by layer.
func checkLayeredBy(t *testing.T, c goldenCase, op edit.Operation, layer string) {
	t.Helper()
	s := open(t, c.fsys, c.layers, c.editLayer, c.pkgs...)
	e, err := s.snap.Editable(resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, op.Path), op.Kind, c.editLayer)
	if err != nil || e.Reason != edit.ReasonLayered || e.Layer != layer {
		t.Errorf("%v: Editable %s: %+v, %v, want layered by %s", c.layers, op.Path, e, err, layer)
	}
	_, err = edit.Apply(t.Context(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	if ne := (*edit.NotEditableError)(nil); !errors.As(err, &ne) || ne.Reason != edit.ReasonLayered || ne.Layer != layer {
		t.Errorf("%v: Apply %s: %v, want layered by %s", c.layers, op.Path, err, layer)
	}
}

// API.md W11, E22 (log-2026-11-01 EDIT-OWED re-review 4): a layer line is judged by what it writes:
// a Reset of the line over a computed base value applies and its Undo writes it back; a new line
// over a spread-supplied value gets one verdict, the edit layer active or not.
func TestLayerLineOverComputedBase(t *testing.T) {
	applyAndUndo(t, goldenCase{fsys: law, pkgs: []string{"a"}, layers: []string{"dev"}, editLayer: "dev"},
		edit.Operation{Kind: edit.OpReset, Path: "a:wrap.code"})
	set := edit.Operation{Kind: edit.OpSet, Path: "a:derived.sub.id", Value: edit.Str("z")}
	for _, c := range []goldenCase{
		{fsys: law, pkgs: []string{"a"}, layers: []string{"dev"}, editLayer: "dev"},
		{fsys: law, pkgs: []string{"a"}, editLayer: "fresh"},
	} {
		s := open(t, c.fsys, c.layers, c.editLayer, c.pkgs...)
		checkVerdict(t, s, resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, set.Path), set, edit.ReasonNone)
		applyAndUndo(t, c, set)
	}
}

// API.md W11 (log-2026-11-01 EDIT-OWED re-review 2): a Reset of a field of an entry only the edit
// layer adds gives the field its default: it leaves the literal; a field with no default is `layer`.
func TestResetFieldOfLayerEntry(t *testing.T) {
	applyAndUndo(t, goldenCase{fsys: keptLine, pkgs: []string{"p"}, layers: []string{"dev"}, editLayer: "dev"},
		edit.Operation{Kind: edit.OpReset, Path: "p:t.three.v"})
	ar, err := txtar.ParseFile(filepath.Join("testdata", "edits", "layer_add_entry_keeps.txtar"))
	if err != nil {
		t.Fatal(err)
	}
	c := readCase(t, ar)
	applyAndUndo(t, c, edit.Operation{Kind: edit.OpReset, Path: "d:quests.hunt.target"})
	s := open(t, c.fsys, c.layers, c.editLayer, c.pkgs...)
	goal := edit.Operation{Kind: edit.OpReset, Path: "d:quests.hunt.goal"}
	checkVerdict(t, s, resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, goal.Path), goal, edit.ReasonLayer)
}

// API.md W11a (log-2026-11-01 EDIT-OWED review): under an edit layer the session does not apply, a
// Set replacing its lines, one re-valuing a key the map has and one adding a key, is `layer` for
// Editable and Apply: its Undo would need an AddEntry no path of the session reaches.
func TestW11aInactiveNewKeyLineRefused(t *testing.T) {
	s := open(t, mapLines, nil, "di", "p")
	op := edit.Operation{Kind: edit.OpSet, Path: "p:byDefault.weights", Value: edit.Source(`{ "b": 2 }`)}
	checkVerdict(t, s, resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, op.Path), op, edit.ReasonLayer)
	one := edit.Operation{Kind: edit.OpSet, Path: "p:byDefault.weights[a]", Value: edit.Int(6)}
	checkVerdict(t, s, resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, one.Path), one, edit.ReasonNone)
}

// API.md 8.2, W5, W11a (DECISIONS 273, 462, 475): under an edit layer the session does not apply,
// an edit inside a typed literal line `Item { … }` is `computed`; a Set of the whole line, a Source
// now, and any edit under the active layer apply, Editable and Apply agreeing, and their Undo restores.
func TestDecisions273TypedLiteralLine(t *testing.T) {
	for _, c := range []struct {
		layers []string
		path   string
		want   edit.Reason
		kind   edit.Op
	}{
		{nil, "a:holder.data", edit.ReasonNone, edit.OpSet},
		{nil, "a:holder.data.n", edit.ReasonComputed, edit.OpSet},
		{[]string{"lt"}, "a:holder.data", edit.ReasonNone, edit.OpSet},
		{[]string{"lt"}, "a:holder.data.n", edit.ReasonNone, edit.OpSet},
		{nil, "a:holder.data", edit.ReasonNone, edit.OpReset},
		{[]string{"lt"}, "a:holder.data", edit.ReasonNone, edit.OpReset},
	} {
		s := open(t, law, c.layers, "lt", "a")
		op := edit.Operation{Kind: c.kind, Path: c.path}
		switch {
		case c.kind == edit.OpReset:
		case c.path == "a:holder.data":
			op.Value = edit.Obj{"id": edit.Str("x")}
		default:
			op.Value = edit.Int(2)
		}
		checkVerdict(t, s, resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, c.path), op, c.want)
		if c.want == edit.ReasonNone {
			applyAndUndo(t, goldenCase{fsys: law, pkgs: []string{"a"}, layers: c.layers, editLayer: "lt"}, op)
		}
	}
}

// pairLines is a record whose field i dev's line states whole, while lx amends i.n below it.
var pairLines = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/p/p.canon": file("/// P.\npackage p\n\n/// I.\nrecord I {\n  /// n.\n  n: Int = 0\n  /// m.\n  m: Int = 0\n}\n\n" +
		"/// W.\nrecord W {\n  /// i.\n  i: I = {}\n}\n\n/// w.\nlet w: W = {}\n"),
	"law/p/lx.layer.canon":  file("package p\nlayer lx\n\namend w {\n  i.n: 5\n}\n"),
	"law/p/dev.layer.canon": file("package p\nlayer dev\n\namend w {\n  i: { n: 2 }\n}\n"),
}

// API.md 7.2 layered, E22 (log-2026-11-01 EDIT-OWED round-6 review): only a true restore, a Set of a
// path and value the edit layer's line held before the request, escapes the layered row; a Reset of
// dev's ancestor line followed by a new Set below it, where lx amends, is `layered` by lx.
func TestLayeredAfterResetUnlessRestore(t *testing.T) {
	ops := []edit.Operation{{Kind: edit.OpReset, Path: "p:w.i"}, {Kind: edit.OpSet, Path: "p:w.i.n", Value: edit.Int(7)}}
	for _, layers := range [][]string{{"dev", "lx"}, {"lx", "dev"}} {
		s := open(t, pairLines, layers, "dev", "p")
		_, err := edit.Apply(t.Context(), s.env, s.snap, edit.Request{Ops: ops})
		if ne := (*edit.NotEditableError)(nil); !errors.As(err, &ne) || ne.Reason != edit.ReasonLayered || ne.Layer != "lx" {
			t.Errorf("%v: Reset p:w.i, Set p:w.i.n: %v, want layered by lx", layers, err)
		}
	}
}
