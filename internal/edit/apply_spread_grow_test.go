package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// spreadGrowSrc has lets that a spread of base gives tags, lim, nodes, tiers, sh and extra, at a
// depth, by chains, beside a stated nodes, and shapes whose case literal spreads c0.
const spreadGrowSrc = `package d

/// Tier.
record Tier {
  /// X.
  x: Int = 1
}

/// Node.
record Node {
  /// Name.
  name: String
  /// V.
  v: Int = 0
}

/// A shape.
variant Shape {
  /// A circle.
  circle {
    /// Radius.
    r: Int = 1
    /// Tags.
    tags: [String] = []
  }
  /// A square.
  square {
    /// Side.
    side: Int = 1
  }
}

/// Box.
record Box {
  /// Tags.
  tags: [String] = []
  /// Lim.
  lim: {String: Int} = {}
  /// Nodes.
  nodes: [Node] keyed by name = []
  /// Tiers.
  tiers: table Tier = {}
  /// Shape.
  sh: Shape = circle {}
  /// Extra, which no layer amends.
  extra: [String] = []
  /// Label.
  label: String = ""
}

/// Holder.
record Holder {
  /// Box.
  box: Box = {}
}

/// The base.
let base: Box = {
  tags: ["p", "q"]
  lim: { "k": 1 }
  nodes: [{ name: "a" }, { name: "b", v: 1 }]
  tiers: {
    /// Low.
    low { x: 3 }
  }
  sh: circle { r: 2 }
  extra: ["e"]
  label: "base"
}

/// Derived.
let derived: Box = { ...base, label: "d" }

/// Derived with nodes stated.
let over: Box = { ...base, nodes: [{ name: "a", v: 5 }] }

/// Deep.
let deep: Holder = { box: { ...base } }

/// Spreading deep's box, which above replaces.
let fromDeep: Box = { ...deep.box }

/// Two spreads away.
let chain: Box = { ...derived }

/// A circle.
let c0: Shape.circle = circle { r: 3, tags: ["t"] }

/// A box whose shape spreads c0.
let cs: Box = { sh: circle { ...c0 } }

/// A box whose shape spreads c0 and states r.
let cr: Box = { sh: circle { ...c0, r: 4 } }

/// Other.
let other: Box = {}
`

// spreadGrowE spreads base across packages, then once more.
const spreadGrowE = "/// E.\npackage e\n\nimport d\n\n/// Cross.\nlet ed: d.Box = { ...d.base, label: \"e\" }\n\n/// Two hops across.\nlet ech: d.Box = { ...ed }\n"

const (
	spreadGrowFile = "law/d/d.canon"
	spreadGrowHot  = "package d\nlayer hot\n\namend base {\n  tags[0]: \"h\"\n  lim[\"k\"]: 7\n  nodes[a].v: 4\n  tiers.low.x: 8\n  sh.r: 5\n}\n\n" +
		"amend c0 {\n  r: 6\n  tags[0]: \"h\"\n}\n"
	spreadGrowAbove = "package d\nlayer above\n\namend deep {\n  box: { tags: [\"A\"] }\n}\n"
)

// spreadGrowPkgs are the packages every session of these tests analyzes.
var spreadGrowPkgs = []string{"d", "e"}

func spreadGrowFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), spreadGrowFile: file(spreadGrowSrc), "law/e/e.canon": file(spreadGrowE),
		"law/d/dev.layer.canon": file("package d\nlayer dev\n\namend other {\n  label: \"o\"\n}\n"),
		"law/d/hot.layer.canon": file(spreadGrowHot), "law/d/above.layer.canon": file(spreadGrowAbove),
	}
}

// spreadGrowRoots are the values a spread of base gives its fields: directly, below a field, through
// a field of a spread let, two spreads away, beside a stated field, across packages and once more.
var spreadGrowRoots = []string{"d:derived", "d:deep.box", "d:fromDeep", "d:chain", "d:over", "e:ed", "e:ech"}

// spreadGrowEntryOps is where spreadGrowOps' AddEntry ops start.
const spreadGrowEntryOps = 3

// spreadGrowOps are the ops that write into a field of root a spread supplies: three into a list,
// then an AddEntry into a map and one into a table.
func spreadGrowOps(root string) []edit.Operation {
	return []edit.Operation{
		{Kind: edit.OpAdd, Path: root + ".tags", Value: edit.Str("n")},
		{Kind: edit.OpInsert, Path: root + ".tags", Index: 0, Value: edit.Str("n")},
		{Kind: edit.OpInsert, Path: root + ".tags", Index: 2, Value: edit.Str("n")},
		{Kind: edit.OpAddEntry, Path: root + ".lim", Key: edit.Str("n"), Value: edit.Int(3)},
		{Kind: edit.OpAddEntry, Path: root + ".tiers", Key: edit.Key("hi"), Value: edit.Obj{"x": edit.Int(3)}},
	}
}

// spreadGrowCases are SetCase ops on path: to another case, and to its own case.
func spreadGrowCases(path string) []edit.Operation {
	return []edit.Operation{
		{Kind: edit.OpSetCase, Path: path, Case: "square", Value: edit.Obj{"side": edit.Int(4)}},
		{Kind: edit.OpSetCase, Path: path, Case: "circle", Value: edit.Obj{"r": edit.Int(7)}},
	}
}

// spreadCaseOps are the ops on a case literal spreading c0: SetCase of cs.sh and cr.sh, and the
// grow ops into the tags their spread supplies.
func spreadCaseOps() []edit.Operation {
	ops := append(spreadGrowCases("d:cs.sh"), spreadGrowCases("d:cr.sh")...)
	return append(ops,
		edit.Operation{Kind: edit.OpAdd, Path: "d:cs.sh.tags", Value: edit.Str("n")},
		edit.Operation{Kind: edit.OpInsert, Path: "d:cs.sh.tags", Index: 0, Value: edit.Str("n")},
		edit.Operation{Kind: edit.OpAdd, Path: "d:cr.sh.tags", Value: edit.Str("n")},
	)
}

// spreadGrowVerdicts is Editable's and Apply's verdict on op in s, Apply's error when it refuses
// with no NotEditableError, and its plan when it applies.
func spreadGrowVerdicts(t *testing.T, s session, op edit.Operation) (edit.Editability, *edit.NotEditableError, *edit.Plan) {
	t.Helper()
	e, err := s.snap.Editable(resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, op.Path), op.Kind, s.env.EditLayer)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	var ne *edit.NotEditableError
	switch {
	case err == nil:
		return e, nil, plan
	case errors.As(err, &ne):
		return e, ne, nil
	}
	t.Fatalf("%s %s: %v", opName(op), op.Path, err)
	return e, nil, nil
}

// spreadGrowRefused reports each op of ops that Editable or Apply, layers active, does not refuse
// `layered` naming layer with no way out.
func spreadGrowRefused(t *testing.T, layers []string, layer string, ops []edit.Operation) {
	t.Helper()
	for _, op := range ops {
		e, ne, _ := spreadGrowVerdicts(t, open(t, spreadGrowFS(), layers, "", spreadGrowPkgs...), op)
		if e.Reason != edit.ReasonLayered || e.Layer != layer || ne == nil || ne.Reason != edit.ReasonLayered || ne.Layer != layer || ne.Detail != "" {
			t.Errorf("%s %s under %v: Editable %v (%q), Apply %+v; want both layered naming %s, no way out", opName(op), op.Path, layers, e.Reason, e.Layer, ne, layer)
		}
	}
}

// DECISIONS 466 (amended), API.md W10, E22: under hot, a base Add, Insert or AddEntry into a field a
// spread supplies (directly, at a depth, by chains, beside a stated field, across packages) whose source
// hot amends is `layered` naming hot, for Editable and Apply alike: its override would hide hot.
func TestSpreadGrowLayered(t *testing.T) {
	var ops []edit.Operation
	for _, root := range spreadGrowRoots {
		ops = append(ops, spreadGrowOps(root)...)
		if root != "d:over" { // over states nodes: no spread supplies it
			ops = append(ops, edit.Operation{Kind: edit.OpAdd, Path: root + ".nodes", Value: edit.Obj{"name": edit.Str("c")}})
		}
	}
	spreadGrowRefused(t, []string{"hot"}, "hot", ops)
}

// DECISIONS 466 (amended), 467, API.md W10, E14, E22: a SetCase writes the whole field a spread supplies,
// or the whole case literal beside a spread of c0, so under hot it is `layered` naming hot, as is a grow
// op into the case's spread-supplied tags: an Undo setting the old value back would hide hot.
func TestSpreadSetCaseLayered(t *testing.T) {
	var ops []edit.Operation
	for _, root := range spreadGrowRoots {
		ops = append(ops, spreadGrowCases(root+".sh")...)
	}
	spreadGrowRefused(t, []string{"hot"}, "hot", append(ops, spreadCaseOps()...))
}

// DECISIONS 466, API.md W10: with above replacing deep.box, a line above every field fromDeep's spread
// supplies, each grow op and SetCase on fromDeep is `layered` naming above.
func TestSpreadGrowLayerAbove(t *testing.T) {
	ops := append(spreadGrowOps("d:fromDeep"), spreadGrowCases("d:fromDeep.sh")...)
	ops = append(ops, edit.Operation{Kind: edit.OpAdd, Path: "d:fromDeep.nodes", Value: edit.Obj{"name": edit.Str("c")}})
	spreadGrowRefused(t, []string{"above"}, "above", ops)
}

// API.md 7.2 (`computed` before `layered`), DECISIONS 466: a Move names an item below a field a spread
// supplies, off the source tree, so it is `computed` whatever the layers, or `layered` naming hot where
// hot sets the item itself; never applied, and Editable and Apply agree.
func TestSpreadMoveRefused(t *testing.T) {
	for _, c := range []struct {
		layers []string
		op     edit.Operation
		want   edit.Reason
		layer  string
	}{
		{nil, edit.Operation{Kind: edit.OpMove, Path: "d:derived.tags[1]"}, edit.ReasonComputed, ""},
		{[]string{"hot"}, edit.Operation{Kind: edit.OpMove, Path: "d:derived.tags[1]"}, edit.ReasonComputed, ""},
		{[]string{"hot"}, edit.Operation{Kind: edit.OpMove, Path: "d:chain.tags[1]"}, edit.ReasonComputed, ""},
		{[]string{"hot"}, edit.Operation{Kind: edit.OpMove, Path: "d:derived.tags[0]", Index: 1}, edit.ReasonLayered, "hot"},
		{[]string{"hot"}, edit.Operation{Kind: edit.OpMove, Path: "d:deep.box.nodes[b]"}, edit.ReasonComputed, ""},
		{[]string{"hot"}, edit.Operation{Kind: edit.OpMove, Path: `d:derived.lim["k"]`}, edit.ReasonLayered, "hot"},
	} {
		e, ne, _ := spreadGrowVerdicts(t, open(t, spreadGrowFS(), c.layers, "", spreadGrowPkgs...), c.op)
		if e.Reason != c.want || e.Layer != c.layer || ne == nil || ne.Reason != c.want || ne.Layer != c.layer {
			t.Errorf("Move %s under %v: Editable %v (%q), Apply %+v; want both %v naming %q", c.op.Path, c.layers, e.Reason, e.Layer, ne, c.want, c.layer)
		}
	}
}

// DECISIONS 462, 466, API.md W5: under an edit layer an Add is `layer`; "make the edit without an
// edit layer" prints only when the base Add passes: with no layer active, not with hot amending the
// spread's source, where the base Add is `layered`.
func TestSpreadGrowWayOut(t *testing.T) {
	op := edit.Operation{Kind: edit.OpAdd, Path: "d:derived.tags", Value: edit.Str("n")}
	for _, c := range []struct {
		layers []string
		way    string
	}{{nil, edit.WayNoLayer}, {[]string{"hot"}, ""}} {
		e, ne, _ := spreadGrowVerdicts(t, open(t, spreadGrowFS(), c.layers, "dev", spreadGrowPkgs...), op)
		if e.Reason != edit.ReasonLayer || e.Way() != c.way || ne == nil || ne.Reason != edit.ReasonLayer || ne.Detail != c.way {
			t.Errorf("Add derived.tags under %v, edit layer dev: Editable %v (%q), Apply %+v; want both layer, way out %q", c.layers, e.Reason, e.Way(), ne, c.way)
		}
	}
}
