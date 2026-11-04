package edit_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/eval"
)

// bareSrc holds the reviewer's shapes: a let spreading a root dev amends, a keyed list and a table
// dev addresses by position.
const bareSrc = `package d

/// A server.
record Server {
  /// Port.
  port: Int = 1
  /// Host.
  host: String = "localhost"
  /// Name.
  name: String = "s"
}

/// A holder of a server.
record Holder {
  /// Its server.
  server: Server = {}
}

/// An item.
record It {
  /// Its id.
  id: String
  /// A weight.
  w: Int = 0
}

/// An entry.
record E {
  /// A value.
  v: Int = 0
}

/// A holder.
record Box {
  /// Items.
  items: [It] keyed by id = []
  /// Rows.
  t: table E = {}
}

/// The base server.
let base: Server = {}

/// A server spreading it.
let derived: Server = { ...base, port: 2 }

/// A server spreading it below a field.
let deep: Holder = { server: { ...base } }

/// Two spreads away from base.
let hop2: Server = { ...derived }

/// Three spreads away from base.
let hop3: Server = { ...hop2 }

/// A nested spread of a nested spread.
let nest: Holder = { server: { ...deep.server } }

/// A shape.
variant Shape {
  /// A circle.
  circle {
    /// Radius.
    r: Int = 1
  }
  /// A square.
  square {
    /// Side.
    side: Int = 1
  }
}

/// A holder dev replaces parts of whole.
record Hold {
  /// Weights.
  m: {String: Int} = {}
  /// A server.
  s: Server = {}
  /// A shape.
  sh: Shape = circle {}
}

/// The held values.
let held: Hold = { m: { "a": 1, "b": 2 }, s: { port: 4 }, sh: circle { r: 2 } }

/// The box.
let box: Box = {
  items: [{ id: "x" }, { id: "y" }]
  t: {
    a { v: 1 }
    b { v: 2 }
  }
}
`

const (
	bareDev = "package d\nlayer dev\n\namend base {\n  host: \"devhost\"\n}\n\namend box {\n  items[#0].w: 9\n  t[#1].v: 9\n}\n\n" +
		"amend held {\n  m: { \"c\": 3 }\n  s: { host: \"h\" }\n  sh: square {}\n}\n"
	bareQA   = "package d\nlayer qa\n\namend base {\n  port: 7\n}\n"
	bareFile = "law/d/d.canon"
)

func bareFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), bareFile: file(bareSrc),
		"law/d/dev.layer.canon": file(bareDev), "law/d/qa.layer.canon": file(bareQA)}
}

// noLayerValue reports an operation's value or Undo free of every value dev states.
func noLayerValue(ops []edit.Operation) bool {
	return !slices.ContainsFunc(ops, func(op edit.Operation) bool {
		text := fmt.Sprint(op.Value, " ", op.Key)
		return strings.Contains(text, "devhost") || strings.Contains(text, "w: 9") || strings.Contains(text, "v: 9")
	})
}

// API.md W9, E22, E23 (EDIT-MATERIALIZE ruling): a base Set, Reset or Remove of a field a spread
// supplies, or a whole-record Set beside it, is `layered` naming the layer when an active layer
// amends such a field on the spread's source, for Editable and Apply alike: its fields go explicit.
func TestBareSpreadFromLayeredRoot(t *testing.T) {
	cases := []struct {
		layers []string
		op     edit.Operation
		want   edit.Reason
		layer  string
	}{
		{[]string{"dev"}, edit.Operation{Kind: edit.OpSet, Path: "d:derived.host", Value: edit.Str("x")}, edit.ReasonLayered, "dev"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpReset, Path: "d:derived.host"}, edit.ReasonLayered, "dev"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpRemove, Path: "d:derived.host"}, edit.ReasonLayered, "dev"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpSet, Path: "d:deep.server.host", Value: edit.Str("x")}, edit.ReasonLayered, "dev"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpReset, Path: "d:deep.server.host"}, edit.ReasonLayered, "dev"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpRemove, Path: "d:deep.server.host"}, edit.ReasonLayered, "dev"},
		{[]string{"dev", "qa"}, edit.Operation{Kind: edit.OpSet, Path: "d:deep.server.port", Value: edit.Int(3)}, edit.ReasonLayered, "qa"},
		{[]string{"dev", "qa"}, edit.Operation{Kind: edit.OpReset, Path: "d:deep.server.port"}, edit.ReasonLayered, "qa"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpSet, Path: "d:hop2.host", Value: edit.Str("x")}, edit.ReasonLayered, "dev"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpSet, Path: "d:hop3.host", Value: edit.Str("x")}, edit.ReasonLayered, "dev"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpReset, Path: "d:hop3.host"}, edit.ReasonLayered, "dev"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpSet, Path: "d:nest.server.host", Value: edit.Str("x")}, edit.ReasonLayered, "dev"},
		{[]string{"dev", "qa"}, edit.Operation{Kind: edit.OpSet, Path: "d:hop2.port", Value: edit.Int(3)}, edit.ReasonNone, ""},
		{[]string{"dev", "qa"}, edit.Operation{Kind: edit.OpSet, Path: "d:nest.server.port", Value: edit.Int(3)}, edit.ReasonLayered, "qa"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpSet, Path: "d:derived", Value: edit.Source("{ port: 3 }")}, edit.ReasonLayered, "dev"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpSet, Path: "d:deep.server", Value: edit.Source("{ port: 3 }")}, edit.ReasonLayered, "dev"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpSet, Path: "d:hop3", Value: edit.Source("{ port: 3 }")}, edit.ReasonLayered, "dev"},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpReset, Path: "d:deep.server"}, edit.ReasonLayered, "dev"},
	}
	for _, c := range cases {
		s := open(t, bareFS(), c.layers, "", "d")
		e, err := s.snap.Editable(resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, c.op.Path), c.op.Kind, "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		var ne *edit.NotEditableError
		applied := c.want == edit.ReasonNone && err == nil
		if e.Reason != c.want || e.Layer != c.layer || !applied && (!errors.As(err, &ne) || ne.Reason != c.want || ne.Layer != c.layer) {
			t.Errorf("%v %d %s: Editable %v (%s), Apply %v; want %v naming %s", c.layers, c.op.Kind, c.op.Path, e.Reason, e.Layer, err, c.want, c.layer)
		}
	}
}

// API.md W9, E22, E23 (EDIT-MATERIALIZE ruling): a spread-supplied field no active layer amends is
// edited as before, an override written; its Undo gives every value back with the layers and without.
func TestBareSpreadUnamendedField(t *testing.T) {
	for _, c := range []struct {
		layers []string
		op     edit.Operation
	}{
		{[]string{"dev"}, edit.Operation{Kind: edit.OpSet, Path: "d:derived.name", Value: edit.Str("x")}},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpSet, Path: "d:deep.server.port", Value: edit.Int(3)}},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpReset, Path: "d:deep.server.port"}},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpSet, Path: "d:hop3.name", Value: edit.Str("x")}},
		{[]string{"dev"}, edit.Operation{Kind: edit.OpSet, Path: "d:nest.server.name", Value: edit.Str("x")}},
	} {
		s := open(t, bareFS(), c.layers, "", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		if err != nil {
			t.Errorf("%v %d %s: %v, want it applied", c.layers, c.op.Kind, c.op.Path, err)
			continue
		}
		f1 := written(bareFS(), plan)
		s1 := open(t, f1, c.layers, "", "d")
		up, err := edit.Apply(context.Background(), s1.env, s1.snap, edit.Request{Ops: plan.Undo})
		if err != nil {
			t.Errorf("%v %d %s, Undo: %v", c.layers, c.op.Kind, c.op.Path, err)
			continue
		}
		f2 := written(f1, up)
		for _, ls := range [][]string{c.layers, nil} {
			want, got := letTexts(t, open(t, bareFS(), ls, "", "d").snap), letTexts(t, open(t, f2, ls, "", "d").snap)
			if !maps.Equal(got, want) {
				t.Errorf("%v %d %s, Undo, layers %v: %v, want %v", c.layers, c.op.Kind, c.op.Path, ls, got, want)
			}
		}
	}
}

// API.md E22, E23 (EDIT-MATERIALIZE ruling): with dev naming box.items[#0], a base Remove of that
// item has an Undo carrying the base's item, not dev's w: 9, and giving every value back.
func TestBareRemoveKeyedByPosition(t *testing.T) {
	op := edit.Operation{Kind: edit.OpRemove, Path: "d:box.items[x]"}
	plan, _, ok := applyIn(t, "Remove box.items[x]", bareFS(), undoView{[]string{"dev"}, ""}, []edit.Operation{op})
	if !ok {
		return
	}
	if !noLayerValue(plan.Undo) {
		t.Errorf("Remove box.items[x] under dev: Undo %+v carries dev's w: 9", plan.Undo)
	}
	undoIn(t, undoSpec{name: "Remove box.items[x]", fsys: bareFS(), view: undoView{[]string{"dev"}, ""}, ops: []edit.Operation{op},
		views: [][]string{{"dev"}, nil}, files: []string{bareFile}})
}

// API.md E22, W10 (EDIT-MATERIALIZE ruling): with dev naming box.t[#1], a base Remove of t[a] is
// refused `layered`: its Undo re-adds a at the end, where dev's position names it, and the Move
// putting it back is judged in that state, after the AddEntry before it.
func TestBareRemoveUndoMoveJudged(t *testing.T) {
	_, ap, _ := guardVerdict(t, bareFS(), edit.Operation{Kind: edit.OpRemove, Path: "d:box.t[a]"})
	if ap != edit.ReasonLayered {
		t.Errorf("Remove box.t[a] under dev: Apply %v, want layered (its Undo's Move)", ap)
	}
}

// API.md W8, W10 (EDIT-MATERIALIZE ruling): the state a base edit plans on under [dev] and [dev,
// qa] holds, root by root, exactly what the project analyzed with no layer holds.
func TestBarePlanBaseIsNoLayer(t *testing.T) {
	want := letTexts(t, open(t, bareFS(), nil, "", "d").snap)
	for _, layers := range [][]string{{"dev"}, {"dev", "qa"}} {
		s := open(t, bareFS(), layers, "", "d")
		base, err := edit.PlanBase(context.Background(), s.env, s.snap)
		if err != nil {
			t.Fatal(err)
		}
		if got := letTexts(t, base); !maps.Equal(got, want) {
			t.Errorf("layers %v: the plan's base %v, want the no-layer analysis %v", layers, got, want)
		}
		if got := letTexts(t, s.snap); maps.Equal(got, want) {
			t.Errorf("layers %v: the session equals the base: the fixture tests nothing", layers)
		}
	}
}

// letTexts is the canonical text of every let of package d in s.
func letTexts(t *testing.T, s *edit.Snapshot) map[string]string {
	t.Helper()
	a := s.Analysis()
	out := map[string]string{}
	for _, pkg := range a.Program().Packages {
		for _, obj := range pkg.Decls {
			if obj.Kind() != check.ObjLet {
				continue
			}
			if v, ok := a.Force(eval.Root{Pkg: pkg.Path, Name: obj.Name()}); ok {
				out[pkg.Path+":"+obj.Name()] = v.CanonText()
			}
		}
	}
	return out
}

// API.md W10, W5 (EDIT-MATERIALIZE ruling): below held.m, held.s or held.sh, which dev replaces
// whole, a base edit is `layered` naming dev, as Editable judges the deepest prefix the session
// resolves, a key or a case field dev's value lacks included.
func TestBareBelowReplacedValue(t *testing.T) {
	for _, c := range []struct {
		op     edit.Operation
		prefix string
	}{
		{edit.Operation{Kind: edit.OpSet, Path: "d:held.m[a]", Value: edit.Int(5)}, "d:held.m"},
		{edit.Operation{Kind: edit.OpRemove, Path: "d:held.m[a]"}, "d:held.m"},
		{edit.Operation{Kind: edit.OpAddEntry, Path: "d:held.m", Key: edit.PathKey("d"), Value: edit.Int(4)}, "d:held.m"},
		{edit.Operation{Kind: edit.OpSet, Path: "d:held.s.port", Value: edit.Int(9)}, "d:held.s.port"},
		{edit.Operation{Kind: edit.OpSet, Path: "d:held.sh.r", Value: edit.Int(3)}, "d:held.sh"},
	} {
		s := open(t, bareFS(), []string{"dev"}, "", "d")
		e, err := s.snap.Editable(resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, c.prefix), c.op.Kind, "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		var ne *edit.NotEditableError
		if e.Reason != edit.ReasonLayered || e.Layer != "dev" || !errors.As(err, &ne) || ne.Reason != edit.ReasonLayered || ne.Layer != "dev" {
			t.Errorf("%d %s under dev: Editable of %s %v (%s), Apply %v; want layered naming dev", c.op.Kind, c.op.Path, c.prefix, e.Reason, e.Layer, err)
		}
	}
}

// crossA is a package whose server dev amends; crossB spreads it, qualified, across packages.
const (
	crossA = "/// A.\npackage a\n\n/// A server.\nrecord Server {\n  /// Port.\n  port: Int = 1\n  /// Host.\n  host: String = \"localhost\"\n}\n\n/// The base server.\nlet base: Server = {}\n"
	crossB = "/// B.\npackage b\n\nimport a\n\n/// A server spreading a's.\nlet derived: a.Server = { ...a.base, port: 2 }\n\n/// Two hops.\nlet hop2: a.Server = { ...derived }\n"
)

// API.md W9, E22 (EDIT-MATERIALIZE ruling): a field a package-qualified spread supplies from
// another package's root dev amends is `layered` there and two hops on, naming dev.
func TestBareSpreadAcrossPackages(t *testing.T) {
	fs := mapFS{"law/project.canon": file(projectCanon), "law/a/a.canon": file(crossA), "law/b/b.canon": file(crossB),
		"law/a/dev.layer.canon": file("package a\nlayer dev\n\namend base {\n  host: \"devhost\"\n}\n")}
	for _, path := range []string{"b:derived.host", "b:hop2.host"} {
		op := edit.Operation{Kind: edit.OpSet, Path: path, Value: edit.Str("x")}
		s := open(t, fs, []string{"dev"}, "", "b")
		e, err := s.snap.Editable(resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, path), op.Kind, "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
		var ne *edit.NotEditableError
		if e.Reason != edit.ReasonLayered || e.Layer != "dev" || !errors.As(err, &ne) || ne.Reason != edit.ReasonLayered {
			t.Errorf("Set %s under dev: Editable %v (%s), Apply %v; want layered naming dev", path, e.Reason, e.Layer, err)
		}
	}
}

// brokenSrc is a let whose only line of dev is in error (E3204): with dev active, bd computes nothing.
const brokenSrc = "package d\n\n/// A record.\nrecord Bd {\n  /// N.\n  n: Int(1..) = 1\n  /// K.\n  k: Int = 0\n}\n\n/// It.\nlet bd: Bd = {}\n"

// API.md W10 (EDIT-MATERIALIZE ruling): with dev active and its line on bd.n in error, the session
// resolves no prefix of bd.n, so no Editable verdict exists; a base Set of bd.n is still `layered`,
// naming dev, judged by dev's lines, while bd.k, which no line names, applies.
func TestBareLayerLineInError(t *testing.T) {
	fs := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(brokenSrc),
		"law/d/dev.layer.canon": file("package d\nlayer dev\n\namend bd {\n  n: 0\n}\n")}
	s := open(t, fs, []string{"dev"}, "", "d")
	if err := resolveErr(t, fixture{Snapshot: s.snap, files: s.a.Files()}, "d:bd.n"); err == nil {
		t.Fatal("d:bd.n resolves with dev active: the fixture tests nothing")
	}
	for _, c := range []struct {
		op   edit.Operation
		want edit.Reason
	}{
		{edit.Operation{Kind: edit.OpSet, Path: "d:bd.n", Value: edit.Int(5)}, edit.ReasonLayered},
		{edit.Operation{Kind: edit.OpReset, Path: "d:bd.n"}, edit.ReasonLayered},
		{edit.Operation{Kind: edit.OpSet, Path: "d:bd.k", Value: edit.Int(3)}, edit.ReasonNone},
	} {
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		var ne *edit.NotEditableError
		switch {
		case c.want == edit.ReasonNone && err != nil:
			t.Errorf("%d %s under dev: %v, want it applied", c.op.Kind, c.op.Path, err)
		case c.want != edit.ReasonNone && (!errors.As(err, &ne) || ne.Reason != c.want || ne.Layer != "dev"):
			t.Errorf("%d %s under dev: %v, want layered naming dev", c.op.Kind, c.op.Path, err)
		}
	}
}
