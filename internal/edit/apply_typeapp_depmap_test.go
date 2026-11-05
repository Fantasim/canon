package edit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// branchSrc's type functions have two record branches whose defaults differ, so a value typed
// against the wrong one is printed without the member that tells them apart (TYPES.md 11.4,
// 11.5): by an earlier field (Pick), by a dependent map's key (Hold).
const branchSrc = `package d

/// A kind.
enum K { a, b }

/// A.
record RA {
  /// X.
  x: Int = 0
}

/// B.
record RB {
  /// X.
  x: Int = 5
}

/// What a kind holds.
type Pick(k: K) = match k {
  a => RA
  b => RB
}

/// A dependent field.
record OT {
  /// Its kind.
  k: K = a
  /// What it holds.
  t: Pick(k) = {}
}

/// Holds an OT.
record HT {
  /// The holder.
  h: OT = {}
}

/// A definition.
record Def {
  /// Its kind.
  k: K
}

/// The definitions.
let defs: table Def = { da { k: a }, db { k: b } }

/// What a definition holds.
type Hold(d: Def) = match d.k {
  a => RA
  b => RB
}

/// A dependent map.
record DM {
  /// Per definition.
  m: {d in defs: Hold(d)} = {}
}

/// A list of dependent maps.
record LO {
  /// The maps.
  l: [DM] = []
}

/// Written in Canon, on the first branch: a brace literal takes it (DECISIONS 171).
let cm: DM = { m: { da: { x: 1 } } }

/// Written in Canon, holding the first branch's default.
let cz: DM = { m: { da: { x: 0 } } }

/// An optional driver.
record OO {
  /// Its kind.
  k: K?
  /// What it holds.
  t: Pick(k)?
  /// A list of them.
  l: [Pick(k)] = []
}

/// Written in Canon, its driver none.
let cn: OO = {}

/// Written in Canon, its driver b, no value.
let cb: OO = { k: b }

/// Shapes of kind a.
variant SA {
  /// A dot.
  dot {
    /// N.
    n: Int = 0
  }
}

/// Shapes of kind b.
variant SB {
  /// A square.
  sq {
    /// N.
    n: Int = 5
  }
}

/// A shape, by kind.
type Form(k: K) = match k {
  a => SA
  b => SB
}

/// A dependent case.
record OC {
  /// Its kind.
  k: K?
  /// Its shape.
  s: Form(k)?
}

/// Written in Canon, its driver none.
let vn: OC = {}

/// Written in Canon on the second branch, by a typed literal.
let vb: OC = { k: b, s: SB.sq { n: 1 } }

/// Written in Canon, empty.
let ce: DM = {}

/// Written in Canon on the second branch, by a typed literal.
let ck: HT = { h: { k: b, t: RB { x: 1 } } }

/// Written in Canon on the second branch of a map, by a typed literal.
let cq: DM = { m: { db: RB { x: 1 } } }

/// Written in Canon, by a field.
let ch: HT = { h: { t: { x: 1 } } }

/// Loaded.
let jm: DM = load("m.json")

/// Loaded, holding the first branch's default.
let jz: DM = load("z.json")

/// Loaded, without db.
let jn: DM = load("n.json")

/// Loaded, a list of maps.
let jo: LO = load("o.json")

/// Loaded, by a field.
let jh: HT = load("h.json")
`

func branchFS() mapFS {
	return mapFS{
		"law/project.canon": file(projectCanon), "law/d/d.canon": file(branchSrc),
		"law/d/m.json": file("{\n  \"m\": {\n    \"db\": {\n      \"x\": 1\n    }\n  }\n}\n"),
		"law/d/z.json": file("{\n  \"m\": {\n    \"db\": {\n      \"x\": 0\n    }\n  }\n}\n"),
		"law/d/n.json": file("{\n  \"m\": {\n    \"da\": {\n      \"x\": 1\n    }\n  }\n}\n"),
		"law/d/o.json": file("{\n  \"l\": [\n    {\n      \"m\": {\n        \"db\": {\n          \"x\": 1\n        }\n      }\n    }\n  ]\n}\n"),
		"law/d/h.json": file("{\n  \"h\": {\n    \"k\": \"b\",\n    \"t\": {\n      \"x\": 1\n    }\n  }\n}\n"),
	}
}

// API.md V1, E22; TYPES.md 11.5, 11.6: a value of a dependent map, and a record given a field a
// type function computes, take the branch their key or record computes, never the first one: the
// value written reads back as given (x 0 in RB, not RB's default 5) and the Undo restores it.
func TestDependentBranchNotFirst(t *testing.T) {
	db0 := edit.Obj{"m": edit.Map{{Key: edit.Key("db"), Value: edit.Obj{"x": edit.Int(0)}}}}
	for _, c := range []struct {
		name, root, want string
		op               edit.Operation
	}{
		{"JSON map: Set the root", "jm", "x: 0", setTo("jm", "{ m: { db: { x: 0 } } }")},
		{"JSON map: Set the root, Obj", "jm", "x: 0", setAt("jm", db0)},
		{"JSON map: Set the map", "jm", "x: 0", setTo("jm.m", "{ db: { x: 0 } }")},
		{"JSON map: Set the value", "jm", "x: 0", setTo("jm.m[db]", "{ x: 0 }")},
		{"JSON map: Insert an item holding it", "jo", "x: 0", edit.Operation{Kind: edit.OpInsert, Path: "jo.l", Index: 0, Value: edit.Source("{ m: { db: { x: 0 } } }")}},
		{"JSON map: Insert an item holding it, Obj", "jo", "x: 0", edit.Operation{Kind: edit.OpInsert, Path: "jo.l", Index: 0, Value: db0}},
		{"JSON map: AddEntry", "jn", "x: 0", edit.Operation{Kind: edit.OpAddEntry, Path: "jn.m", Key: edit.Key("db"), Value: edit.Source("{ x: 0 }")}},
		{"JSON map: Reset, held first branch's default", "jz", "{m: {}}", reset("jz.m")},
		{"JSON field: Set the holder", "jh", "x: 0", setTo("jh.h", "{ k: b, t: { x: 0 } }")},
		{"Canon map: Set the root", "cm", "x: 0", setTo("cm", "{ m: { da: { x: 0 } } }")},
		{"Canon map: Set the map", "cm", "x: 0", setTo("cm.m", "{ da: { x: 0 } }")},
		{"Canon map: Set the map, Obj", "cm", "x: 0", setAt("cm.m", edit.Map{{Key: edit.Key("da"), Value: edit.Obj{"x": edit.Int(0)}}})},
		{"Canon map: AddEntry", "ce", "x: 1", edit.Operation{Kind: edit.OpAddEntry, Path: "ce.m", Key: edit.Key("da"), Value: edit.Source("{ x: 1 }")}},
		{"Canon map: Reset, held first branch's default", "cz", "{m: {}}", reset("cz.m")},
		{"Canon field: Set the holder", "ch", "x: 0", setTo("ch.h", "{ t: { x: 0 } }")},
	} {
		edited, ok := applied(t, c.name, branchFS(), []edit.Operation{c.op})
		if !ok {
			continue
		}
		if got := values(open(t, edited.fsys, nil, "", "d"), false)["d:"+c.root]; !strings.Contains(got, c.want) {
			t.Errorf("%s: %s = %s, want it to hold %s", c.name, c.root, got, c.want)
		}
		checkClean(t, c.name, branchFS(), []edit.Operation{c.op})
		undoTwice(t, c.name, branchFS(), []edit.Operation{c.op})
	}
}

// API.md V1 (EDIT-TYPEAPPSET ruling b): bound for a JSON source, a literal whose branch cannot be
// computed is a ValueError, as an Obj is; DECISIONS 171's first branch is Canon text's only.
func TestDependentBranchUncomputedJSON(t *testing.T) {
	for _, c := range []struct {
		name string
		op   edit.Operation
		bad  bool
	}{
		{"JSON map: a key naming no entry", setTo("jm", "{ m: { dzz: { x: 0 } } }"), true},
		{"JSON map: a key naming no entry, the map", setTo("jm.m", "{ dzz: { x: 0 } }"), true},
		{"Canon map: a key naming no entry", setTo("cm", "{ m: { dzz: { x: 0 } } }"), false},
	} {
		s := open(t, branchFS(), nil, "", "d")
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		t.Logf("%s: %v", c.name, err)
		if errors.Is(err, edit.ErrBadValue) != c.bad {
			t.Errorf("%s: %v, want ErrBadValue: %v", c.name, err, c.bad)
		}
	}
}

// API.md V1, E15, E22 (DECISIONS 171, 378, 475): bound for Canon text, a bare brace or list a Source
// writes on a branch the checker does not read it as, and a case in any form, is a ValueError; an
// Obj, List or Map record is printed typed instead (TestDependentSecondBranchTyped).
func TestDependentSecondBranchCanonRefused(t *testing.T) {
	kb := edit.Member("b")
	sq := edit.Case{Name: "sq", Fields: edit.Obj{"n": edit.Int(1)}}
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"optional field, driver set beside it", []edit.Operation{setTo("cn", "{ k: b, t: { x: 1 } }")}},
		{"list element, driver set beside it", []edit.Operation{setTo("cn", "{ k: b, l: [{ x: 1 }] }")}},
		{"required field", []edit.Operation{setTo("ch.h", "{ k: b, t: { x: 1 } }")}},
		{"map value", []edit.Operation{setTo("cm.m", "{ db: { x: 0 } }")}},
		{"top level, driver set before", []edit.Operation{setAt("cn.k", kb), setTo("cn.t", "{ x: 1 }")}},
		{"top level, driver already b", []edit.Operation{setTo("cb.t", "{ x: 1 }")}},
		{"top level, required field, driver set before", []edit.Operation{setAt("ch.h.k", kb), setTo("ch.h.t", "{ x: 1 }")}},
		{"case, driver set beside it", []edit.Operation{setTo("vn", "{ k: b, s: sq { n: 1 } }")}},
		{"case, driver set beside it, Obj", []edit.Operation{setAt("vn", edit.Obj{"k": kb, "s": sq})}},
		{"case, top level, driver set before", []edit.Operation{setAt("vn.k", kb), setAt("vn.s", sq)}},
	} {
		s := open(t, branchFS(), nil, "", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		if !errors.Is(err, edit.ErrBadValue) || plan != nil {
			t.Errorf("%s: planned %v, %v, want ErrBadValue and no plan", c.name, plan != nil, err)
		}
	}
}

// API.md V1, M7, E22 (DECISIONS 475): the rows TestDependentSecondBranchCanonRefused refused as an
// Obj, List or Map before typed literals: each is printed as a typed literal of the branch its
// record computes, checks clean, and its Undo restores.
func TestDependentSecondBranchTyped(t *testing.T) {
	kb, x1 := edit.Member("b"), edit.Obj{"x": edit.Int(1)}
	for _, c := range []typedLitCase{
		{"optional field, driver set beside it, Obj", "cn", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setAt("cn", edit.Obj{"k": kb, "t": x1})}},
		{"list element, driver set beside it, List", "cn", "l: [RB{x: 1}]", "l: [RB { x: 1 }]", []edit.Operation{setAt("cn", edit.Obj{"k": kb, "l": edit.List{x1}})}},
		{"required field, Obj", "ch", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setAt("ch.h", edit.Obj{"k": kb, "t": x1})}},
		{"map value, Map", "cm", "db: RB{x: 1}", "db: RB { x: 1 }", []edit.Operation{setAt("cm.m", edit.Map{{Key: edit.Key("db"), Value: x1}})}},
		{"top level, driver set before, Obj", "cn", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setAt("cn.k", kb), setAt("cn.t", x1)}},
	} {
		typedLitApplies(t, branchFS(), c)
	}
}

// API.md V1, M6, E22 (EDIT-TYPEAPPSET round 5): a value written into an existing typed literal of
// its own type keeps that literal's name (M6), so Canon text reads the branch it computes: the
// Set applies, checks clean, and its Undo restores (E22).
func TestDependentSecondBranchInTypedLiteral(t *testing.T) {
	for _, c := range []struct {
		name, root, want string
		op               edit.Operation
	}{
		{"record", "ck", "RB{x: 2}", setTo("ck.h.t", "{ x: 2 }")},
		{"record, Obj", "ck", "RB{x: 2}", setAt("ck.h.t", edit.Obj{"x": edit.Int(2)})},
		{"case", "vb", "sq{n: 2}", setAt("vb.s", edit.Case{Name: "sq", Fields: edit.Obj{"n": edit.Int(2)}})},
	} {
		edited, ok := applied(t, c.name, branchFS(), []edit.Operation{c.op})
		if !ok {
			continue
		}
		if got := values(open(t, edited.fsys, nil, "", "d"), false)["d:"+c.root]; !strings.Contains(got, c.want) {
			t.Errorf("%s: %s = %s, want it to hold %s", c.name, c.root, got, c.want)
		}
		checkClean(t, c.name, branchFS(), []edit.Operation{c.op})
		undoTwice(t, c.name, branchFS(), []edit.Operation{c.op})
	}
}

// layerFS is branchFS with a table lt and a layer dev amending its entry e1 by a typed literal.
func layerFS() mapFS {
	fs := branchFS()
	fs["law/d/d.canon"] = file(branchSrc + `
/// A layered entry.
record LT {
  /// Its kind.
  k: K = a
  /// What it holds.
  t: Pick(k)?
}

/// The layered table.
let lt: table LT = { e1 { k: b, t: RB { x: 1 } }, e2 {} }
`)
	fs["law/d/dev.layer.canon"] = file("package d\nlayer dev\n\namend lt {\n  e1.t: RB { x: 3 }\n}\n")
	return fs
}

// API.md V1, W10, E22 (DECISIONS 475): under an active edit layer, a Source's bare brace on another
// branch is refused, nothing written (an Obj is a typed line: TestTypedLiteralEditLayer); one on
// the first branch applies and its Undo restores the values.
func TestDependentSecondBranchEditLayer(t *testing.T) {
	kb := edit.Member("b")
	view := undoView{[]string{"dev"}, "dev"}
	for _, ops := range [][]edit.Operation{
		{setAt("lt.e2.k", kb), setTo("lt.e2.t", "{ x: 4 }")},
	} {
		s := open(t, layerFS(), view.layers, view.edit, "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
		if !errors.Is(err, edit.ErrBadValue) || plan != nil {
			t.Errorf("%+v: planned %v, %v, want ErrBadValue and no plan", ops, plan != nil, err)
		}
	}
	for _, c := range []struct {
		name, want string
		op         edit.Operation
	}{
		{"first branch", "e2: LT{k: a, t: RA{x: 4}}", setTo("lt.e2.t", "{ x: 4 }")},
		{"first branch, Obj", "e2: LT{k: a, t: RA{x: 4}}", setAt("lt.e2.t", edit.Obj{"x": edit.Int(4)})},
	} {
		if _, fs, ok := applyIn(t, c.name, layerFS(), view, []edit.Operation{c.op}); ok {
			if got := values(open(t, fs, view.layers, "", "d"), false)["d:lt"]; !strings.Contains(got, c.want) {
				t.Errorf("%s: lt = %s under dev, want it to hold %s", c.name, got, c.want)
			}
		}
		undoIn(t, undoSpec{name: c.name, fsys: layerFS(), view: view, ops: []edit.Operation{c.op}, views: [][]string{{"dev"}, nil}, lines: []string{"law/d/dev.layer.canon"}})
	}
}

// API.md M7, E22, E23 (DECISIONS 171, 475): a Canon value on a record branch a brace does not
// classify to has an Undo written as a typed literal (`RB { … }`): the Undo of a Reset applies and
// restores the value, RB{x: 1} where `t: RB { x: 1 }` was written.
func TestDependentSecondBranchUndoRestores(t *testing.T) {
	for _, c := range []struct {
		root, want string
		op         edit.Operation
	}{
		{"ck", "t: RB{x: 1}", reset("ck.h")},
		{"cq", "db: RB{x: 1}", reset("cq.m")},
		{"ck", "t: RB{x: 1}", reset("ck.h.t")},
	} {
		edited, ok := applied(t, c.op.Path, branchFS(), []edit.Operation{c.op})
		if !ok {
			continue
		}
		back, ok := applied(t, c.op.Path+", Undo", edited.fsys, edited.plan.Undo)
		if !ok {
			continue
		}
		if got := values(open(t, back.fsys, nil, "", "d"), false)["d:"+c.root]; !strings.Contains(got, c.want) {
			t.Errorf("%s, Undo: %s = %s, want it to hold %s", c.op.Path, c.root, got, c.want)
		}
		undoTwice(t, c.op.Path, branchFS(), []edit.Operation{c.op})
	}
}
