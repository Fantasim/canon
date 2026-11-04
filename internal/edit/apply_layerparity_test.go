package edit_test

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// The layer files the parity rows read and rewrite.
const (
	slFile = "law/a/sl.layer.canon"
	fyFile = "law/a/fy.layer.canon"
	njFile = "law/a/nj.layer.canon"
	moFile = "law/m/mo.layer.canon"
	miFile = "law/m/mi.layer.canon"
	mtFile = "law/m/mt.layer.canon"
	muFile = "law/m/mu.layer.canon"
	mlFile = "law/m/ml.layer.canon"
	lzFile = "law/d/lz.layer.canon"
	lcFile = "law/d/lc.layer.canon"
)

// mapLaw is a record holding a map, which layer mo amends with a literal map beside a call, and a
// record whose fields default to an expression and to a literal, which layer mi amends beside a call.
var mapLaw = mapFS{
	"law/project.canon": file(projectCanon),
	"law/m/m.canon": file("/// M.\npackage m\n\n/// An inner record.\nrecord I {\n  /// X.\n  x: Int = 1 + 1\n  /// Y.\n  y: Int = 0\n  /// Z.\n  z: Int = 0\n  /// W, by z.\n  w: Int = z + 1\n}\n\n" +
		"/// A holder.\nrecord M {\n  /// Weights.\n  m: {String: Int} = {}\n  /// N.\n  n: Int = 0\n  /// Inner.\n  inner: I = {}\n  /// Rows.\n  t: table I = {}\n  /// List.\n  l: [Int] = []\n}\n\n" +
		"/// Three.\nfn f() -> Int {\n  return 3\n}\n\n/// The holder.\nlet r: M = { m: { \"x\": 1, \"y\": 1, \"z\": 1 } }\n"),
	"law/m/mo.layer.canon": file("package m\nlayer mo\n\namend r {\n  m: { \"x\": f(), \"y\": 2, \"z\": 3 }\n}\n"),
	"law/m/mi.layer.canon": file("package m\nlayer mi\n\namend r {\n  inner: { y: f() }\n}\n"),
	"law/m/mt.layer.canon": file("package m\nlayer mt\n\namend r {\n  t: { one { y: f() }, two { y: 2 }, three { y: 3 } }\n}\n"),
	"law/m/mu.layer.canon": file("package m\nlayer mu\n\namend r {\n  t: { two { y: 2 }, one { y: f() } }\n}\n"),
	"law/m/ml.layer.canon": file("package m\nlayer ml\n\namend r {\n  l: [f(), 2, 3]\n}\n"),
}

// layerParityLaws are the projects of the parity rows: wayLaw's lines with a literal in place of a load
// or a call (slLit, fyLit, fyE), mapLaw, and rootLaw with a commented line (lc).
func layerParityLaws() map[string]mapFS {
	slLit, _ := afterWay(wayLaw, followed{"sl", jsonLoad, "{ one { v: 2 } }"})
	fyLit, _ := afterWay(wayLaw, followed{"fy", "mk()", "{ one { v: 1 } }"})
	fyE, _ := afterWay(wayLaw, followed{"fy", "e0", "{ v: 9 }"})
	root := rootLaw()
	root[lcFile] = file("package d\nlayer lc\n\namend qs {\n  a: {\n    items: true // c\n    rewards: [\"s\"]\n    note: \"y\"\n  }\n}\n")
	return map[string]mapFS{"way": wayLaw, "slLit": slLit, "fyLit": fyLit, "fyE": fyE, "map": mapLaw, "root": root}
}

// parityRow is an op under an edit layer, the reason Editable and Apply give it in every state
// asked, and for one that applies the layer file its Undo leaves ("" for the bytes before it).
type parityRow struct {
	name, law, layer, file string
	in                     states
	op                     edit.Operation
	reason                 edit.Reason
	back                   string
}

// lineFile is a layer file of package pkg amending root with the one line given.
func lineFile(pkg, layer, root, line string) string {
	return "package " + pkg + "\nlayer " + layer + "\n\namend " + root + " {\n  " + line + "\n}\n"
}

// parityRows are TestLayerParity's cases, by bug: (a) a field the line states equal to its default,
// (b) a field the line leaves out beside a load, a map beside a call, (c) the driver beside an
// identifier, (d) a comment on a field a Set leaves to its default.
var parityRows = []parityRow{
	// (a) The Undo gives back every value; E6 leaves out a field it sets to its default (API.md E22).
	{"a: Set of a table the line states at its default, beside a load", "way", "sl", slFile, inBoth,
		setTo("a:r.w.t2", "{ one { v: 3 } }"), edit.ReasonNone, lineFile("a", "sl", "r", `w: { sub: load("s.json"), e: { v: 2 } }`)},
	{"a: Reset of a table the line states at its default, beside a load", "way", "sl", slFile, inBoth,
		wayOp(edit.OpReset, "a:r.w.t2"), edit.ReasonNone, lineFile("a", "sl", "r", `w: { sub: load("s.json"), e: { v: 2 } }`)},
	{"a: Set inside a table the line states at its default", "way", "sl", slFile, inBoth,
		wayOp(edit.OpSet, "a:r.w.t2[one].v"), edit.ReasonNone, ""},
	{"a: AddEntry into a table the line states at its default, active: Remove of the entry", "way", "sl", slFile, inActive,
		wayOp(edit.OpAddEntry, "a:r.w.t2"), edit.ReasonNone, ""},
	{"a: AddEntry into a table the line states at its default, inactive: Set of the table (W11a)", "way", "sl", slFile, inInactive,
		wayOp(edit.OpAddEntry, "a:r.w.t2"), edit.ReasonNone, lineFile("a", "sl", "r", `w: { sub: load("s.json"), e: { v: 2 } }`)},
	{"a: Set of a table the line states at its default, all literal", "slLit", "sl", slFile, inBoth,
		setTo("a:r.w.t2", "{ one { v: 3 } }"), edit.ReasonNone, lineFile("a", "sl", "r", "w: { sub: { one { v: 2 } }, e: { v: 2 } }")},
	{"a: Set of a table at its default, beside an expression", "fyLit", "fy", fyFile, inBoth,
		setTo("a:r.w.sub", "{ two { v: 3 } }"), edit.ReasonNone, lineFile("a", "fy", "r", "w: { e: e0 }")},
	{"a: Reset of a table at its default, beside an expression", "fyLit", "fy", fyFile, inBoth,
		wayOp(edit.OpReset, "a:r.w.sub"), edit.ReasonNone, lineFile("a", "fy", "r", "w: { e: e0 }")},
	{"a: Reset of a record to its default, beside a call", "fyE", "fy", fyFile, inBoth,
		wayOp(edit.OpReset, "a:r.w.e"), edit.ReasonNone, lineFile("a", "fy", "r", "w: { sub: mk(), e: { v: 9 } }")},
	// (b) A field the line leaves out is inserted, its Undo takes it out again (DECISIONS 462).
	{"b: Set below a field left out beside a load", "way", "nj", njFile, inBoth,
		wayOp(edit.OpSet, "a:r.w.e.v"), edit.ReasonNone, ""},
	{"b: Set of a field left out beside a load", "way", "nj", njFile, inBoth,
		setTo("a:r.w.e", "{ v: 3 }"), edit.ReasonNone, ""},
	// The inserted table broke the line past the width (M5); its removal leaves it broken (FORMATTER.md 7).
	{"b: AddEntry into a table left out beside a load", "way", "nj", njFile, inBoth,
		wayOp(edit.OpAddEntry, "a:r.w.t2"), edit.ReasonNone, lineFile("a", "nj", "r", "w: {\n    sub: load(\"s.json\")\n  }")},
	{"b: Set of a field left out beside a call", "map", "mi", miFile, inBoth,
		edit.Operation{Kind: edit.OpSet, Path: "m:r.inner.z", Value: edit.Int(5)}, edit.ReasonNone, ""},
	{"b: Set to its default of a field left out beside a call writes nothing", "map", "mi", miFile, inBoth,
		edit.Operation{Kind: edit.OpSet, Path: "m:r.inner.z", Value: edit.Int(0)}, edit.ReasonNone, ""},
	// (b) A field left out whose default reads no field: computed from its default alone.
	{"b: Set of a field left out whose default is an expression, beside a call", "map", "mi", miFile, inBoth,
		edit.Operation{Kind: edit.OpSet, Path: "m:r.inner.x", Value: edit.Int(5)}, edit.ReasonNone, ""},
	// (b) One whose default reads another field: the active session has the record, the inactive one
	// not the line's other values (DECISIONS 273's inactive limit, 462 as amended).
	{"b: Set of a field left out whose default reads a field, active", "map", "mi", miFile, inActive,
		edit.Operation{Kind: edit.OpSet, Path: "m:r.inner.w", Value: edit.Int(5)}, edit.ReasonNone, ""},
	{"b: Set of a field left out whose default reads a field, inactive", "map", "mi", miFile, inInactive,
		edit.Operation{Kind: edit.OpSet, Path: "m:r.inner.w", Value: edit.Int(5)}, edit.ReasonComputed, ""},
	// (b) Inactive, a Remove or AddEntry inside a literal Sets the collection (E23), which holds a call:
	// no Source writes it back (DECISIONS 273). Active, its Undo is an AddEntry or a Remove: it applies
	// where that Undo restores, a last entry stating no call (DECISIONS 462 as amended).
	{"b: Remove of a map entry beside a call", "map", "mo", moFile, inBoth,
		edit.Operation{Kind: edit.OpRemove, Path: `m:r.m["y"]`}, edit.ReasonComputed, ""},
	{"b: Remove of the last map entry beside a call, active", "map", "mo", moFile, inActive,
		edit.Operation{Kind: edit.OpRemove, Path: `m:r.m["z"]`}, edit.ReasonNone, ""},
	{"b: Remove of the last map entry beside a call, inactive", "map", "mo", moFile, inInactive,
		edit.Operation{Kind: edit.OpRemove, Path: `m:r.m["z"]`}, edit.ReasonComputed, ""},
	{"b: AddEntry into a map beside a call, active", "map", "mo", moFile, inActive,
		edit.Operation{Kind: edit.OpAddEntry, Path: "m:r.m", Key: edit.Key("w"), Value: edit.Int(4)}, edit.ReasonNone, ""},
	{"b: AddEntry into a map beside a call, inactive", "map", "mo", moFile, inInactive,
		edit.Operation{Kind: edit.OpAddEntry, Path: "m:r.m", Key: edit.Key("w"), Value: edit.Int(4)}, edit.ReasonComputed, ""},
	{"b: Remove of a table entry beside a call, active", "map", "mt", mtFile, inActive,
		edit.Operation{Kind: edit.OpRemove, Path: "m:r.t[two]"}, edit.ReasonComputed, ""},
	{"b: Remove of the last table entry beside a call, active", "map", "mt", mtFile, inActive,
		edit.Operation{Kind: edit.OpRemove, Path: "m:r.t[three]"}, edit.ReasonNone, ""},
	{"b: Remove of a table entry stating a call, active", "map", "mt", mtFile, inActive,
		edit.Operation{Kind: edit.OpRemove, Path: "m:r.t[one]"}, edit.ReasonComputed, ""},
	{"b: Remove of the last table entry, stating a call, active", "map", "mu", muFile, inActive,
		edit.Operation{Kind: edit.OpRemove, Path: "m:r.t[one]"}, edit.ReasonComputed, ""},
	{"b: AddEntry into a table beside a call, active", "map", "mt", mtFile, inActive,
		edit.Operation{Kind: edit.OpAddEntry, Path: "m:r.t", Key: edit.Key("w"), Value: edit.Obj{"y": edit.Int(4)}}, edit.ReasonNone, ""},
	{"b: Remove of the last list element beside a call, active", "map", "ml", mlFile, inActive,
		edit.Operation{Kind: edit.OpRemove, Path: "m:r.l[2]"}, edit.ReasonComputed, ""},
	{"b: Set of a list element beside a call, active", "map", "ml", mlFile, inActive,
		edit.Operation{Kind: edit.OpSet, Path: "m:r.l[1]", Value: edit.Int(9)}, edit.ReasonNone, ""},
	{"b: Set of a map value beside a call", "map", "mo", moFile, inBoth,
		edit.Operation{Kind: edit.OpSet, Path: `m:r.m["y"]`, Value: edit.Int(4)}, edit.ReasonNone, ""},
	// (c) The driver beside an identifier, the layer active: applied; the re-check judges its dependent sibling.
	{"c: Set of the driver beside an identifier", "root", "lz", lzFile, inActive,
		edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].items", Value: edit.Bool(false)}, edit.ReasonNone,
		lineFile("d", "lz", "qs", `a: { items: true, n: k0, rewards: ["s"], note: "y" }`)},
	// (d) A Set to the default removes the field with its comments (E6, M4); E22 restores no comment of a removed item.
	{"d: Set to the default of a commented field", "root", "lc", lcFile, inActive,
		edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].items", Value: edit.Bool(false)}, edit.ReasonNone,
		"package d\nlayer lc\n\namend qs {\n  a: {\n    items: true\n    rewards: [\"s\"]\n    note: \"y\"\n  }\n}\n"},
	{"d: Set beside a commented field keeps the comment", "root", "lc", lcFile, inActive,
		edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].note", Value: edit.Str("z")}, edit.ReasonNone, ""},
}

// API.md W5, W11, W11a, E22 (DECISIONS 273, 462): Editable and Apply give an edit inside an edit
// layer's line the same verdict, the layer active or not; one that applies has an Undo that gives
// every value back, and the layer file the bytes the row names.
func TestLayerParity(t *testing.T) { runParity(t, layerParityLaws(), parityRows) }

// runParity asks each row in each of its states, and follows the Undo of each that applies.
func runParity(t *testing.T, laws map[string]mapFS, rows []parityRow) {
	t.Helper()
	for _, c := range rows {
		for _, layers := range c.in.sessions(c.layer) {
			t.Run(c.name, func(t *testing.T) {
				fsys := laws[c.law]
				got := askWay(t, fsys, layers, c.layer, c.op)
				if want := (wayAnswer{c.reason, c.reason, "", ""}); got != want {
					t.Fatalf("layers %v: Editable %d %q, Apply %d %q, want %d and no way out", layers, got.editReason, got.editWay, got.applyReason, got.applyWay, c.reason)
				}
				if c.reason == edit.ReasonNone {
					undoBack(t, fsys, layers, c)
				}
			})
		}
	}
}

// undoBack applies c's op, then its Undo, and compares the layer file and every value (API.md E22).
func undoBack(t *testing.T, fsys mapFS, layers []string, c parityRow) {
	t.Helper()
	pkg, _, _ := strings.Cut(c.op.Path, ":")
	s := open(t, fsys, layers, c.layer, pkg)
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
	if err != nil {
		t.Fatalf("layers %v: Apply: %v", layers, err)
	}
	after := written(fsys, plan)
	u := open(t, after, layers, c.layer, pkg)
	undo, err := edit.Apply(context.Background(), u.env, u.snap, edit.Request{Ops: plan.Undo})
	if err != nil {
		t.Fatalf("layers %v: Undo %v: %v", layers, plan.Undo, err)
	}
	back := written(after, undo)
	want := c.back
	if want == "" {
		want = fileText(fsys, c.file)
	}
	if got := fileText(back, c.file); got != want {
		t.Errorf("layers %v: after the Undo %v, %s holds:\n%s\nwant:\n%s", layers, plan.Undo, c.file, got, want)
	}
	for _, active := range [][]string{nil, {c.layer}} {
		before, restored := values(open(t, fsys, active, c.layer, pkg), false), values(open(t, back, active, c.layer, pkg), false)
		for _, name := range slices.Sorted(maps.Keys(before)) {
			if restored[name] != before[name] {
				t.Errorf("layers %v, read with %v: the Undo leaves %s = %s, want %s", layers, active, name, restored[name], before[name])
			}
		}
	}
}
