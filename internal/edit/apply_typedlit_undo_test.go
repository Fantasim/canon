package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// undoSrc has values whose Undo Canon text could not state: lists and maps on a second branch,
// given by name, a first-branch case written qualified, an applied record on a second branch.
const undoSrc = `package d

/// A slot by kind.
type Slot(k: K) = match k {
  a => [Int]
  b => [String]
}

/// A bag by kind.
type Bag(k: K) = match k {
  a => {String: Int}
  b => {String: String}
}

/// By kind.
record S {
  /// Its kind.
  k: K = a
  /// A slot.
  s: Slot(k) = []
  /// A bag.
  m: Bag(k) = {}
}

/// Strings.
let strs: [String] = ["x"]

/// A map of strings.
let smap: {String: String} = { "q": "x" }

/// On the second branch, by names.
let s2: S = { k: b, s: strs, m: smap }

/// On the first branch, a case written qualified.
let va: OC = { k: a, s: SA.dot { n: 1 } }

/// Applied by kind.
record PR(k: K) {
  /// X.
  x: Int = 0
}

/// By kind, the second an applied record.
type PickA(k: K) = match k {
  a => RA
  b => PR(k)
}

/// Holds one.
record OA {
  /// Its kind.
  k: K = a
  /// What it holds.
  t: PickA(k)?
}

/// Records by kind.
type Recs(k: K) = match k {
  a => [RA]
  b => [RB]
}

/// Maps by kind.
type Maps(k: K) = match k {
  a => {String: Int}
  b => {String: String}
}

/// Containers by kind.
record RR {
  /// Its kind.
  k: K = a
  /// Records.
  r: Recs(k) = []
  /// A map.
  m: Maps(k) = {}
}

/// Containers on the first branch.
let ra: RR = {}

/// Containers on the second branch.
let rb: RR = { k: b }

/// Loaded, containers on the second branch.
let jr: RR = load("jr.json")

/// An event.
record E5 {
  /// Its kind.
  k: K
}

/// The events.
let evs5: table E5 = { one { k: a }, two { k: b } }

/// A shape by event.
type FormE(e: E5) = match e.k {
  a => SA
  b => SB
}

/// A case driven by a ref, beside another field.
record H5 {
  /// Its event.
  ev: ref evs5?
  /// Its shape.
  s: FormE(ev)?
  /// Another field.
  x: Int = 0
}

/// A first-branch case.
let h5: H5 = { ev: one, s: SA.dot { n: 1 } }

/// Loaded, a case on the second branch.
let vj: OC = load("vj.json")

/// Loaded, an applied record on the second branch.
let ja: OA = load("ja.json")
`

// typedUndoFS is branchFS with undoSrc and its JSON sources.
func typedUndoFS() mapFS {
	fs := branchFS()
	fs["law/d/undo.canon"] = file(undoSrc)
	fs["law/d/vj.json"] = file("{\"k\": \"b\", \"s\": {\"kind\": \"sq\", \"n\": 1}}\n")
	fs["law/d/ja.json"] = file("{\"k\": \"b\", \"t\": {\"x\": 1}}\n")
	fs["law/d/jr.json"] = file("{\"k\": \"b\", \"r\": [{\"x\": 1}], \"m\": {\"q\": \"x\"}}\n")
	return fs
}

// insertR and addM add an item to root's records and map.
func insertR(root string) edit.Operation {
	return edit.Operation{Kind: edit.OpInsert, Path: root + ".r", Index: 0, Value: edit.Obj{"x": edit.Int(2)}}
}

func addM(root string, v edit.Lit) edit.Operation {
	return edit.Operation{Kind: edit.OpAddEntry, Path: root + ".m", Key: edit.Str("z"), Value: v}
}

// API.md V1, E22 (DECISIONS 171, 474): an item into a container of a dependent type whose list or
// map branch is not the one a bare `[…]` or `{…}` reads as is a ValueError bound for Canon text,
// nothing written; on the first branch, or bound for a JSON source, it applies and undoes.
func TestTypedLiteralItemIntoSecondBranch(t *testing.T) {
	refused(t, typedUndoFS(), []namedOps{
		{"Insert, a list on the second branch", []edit.Operation{insertR("rb")}},
		{"AddEntry, a map on the second branch", []edit.Operation{addM("rb", edit.Str("y"))}},
	})
	for _, op := range []edit.Operation{insertR("ra"), addM("ra", edit.Int(1)), insertR("jr"), addM("jr", edit.Str("y"))} {
		checkClean(t, op.Path, typedUndoFS(), []edit.Operation{op})
		undoTwice(t, op.Path, typedUndoFS(), []edit.Operation{op})
	}
}

// API.md E22, V1 (WIRE.md 5.9; DECISIONS 466, 474): an inverse bound for a JSON source is read on
// the branch its record computes, whatever Canon text would read: a case or an applied record there
// is set, reset and restored by its Undo.
func TestTypedLiteralJSONUndo(t *testing.T) {
	for _, op := range []edit.Operation{
		setAt("vj.s", edit.Case{Name: "sq", Fields: edit.Obj{"n": edit.Int(2)}}),
		reset("vj.s"),
		setAt("vj", edit.Obj{}),
		reset("ja.t"),
		setAt("ja", edit.Obj{}),
	} {
		checkClean(t, op.Path, typedUndoFS(), []edit.Operation{op})
		undoTwice(t, op.Path, typedUndoFS(), []edit.Operation{op})
	}
}

// API.md E22, M7 (DECISIONS 171, 378, 466, 474): an edit whose Undo, bound for Canon text, would
// print a list, map or case another branch than its record computes, its own or a cascade's, is
// refused up front with that ValueError, nothing written.
func TestTypedLiteralUndoRefusedUpFront(t *testing.T) {
	refused(t, typedUndoFS(), []namedOps{
		{"a list and a map, whole root", []edit.Operation{setAt("s2", edit.Obj{"k": edit.Member("a")})}},
		{"a case, by the driver's cascade", []edit.Operation{setAt("vb.k", edit.Member("a"))}},
		{"a case, by the driver's cascade to none", []edit.Operation{setAt("vb.k", edit.None{})}},
		{"a first-branch case written qualified, whole root", []edit.Operation{setAt("va", edit.Obj{})}},
	})
	for _, ops := range [][]edit.Operation{
		{setAt("ck.h.t.x", edit.Int(3)), setAt("vb.k", edit.Member("a"))},
		{setAt("ck.h.t.x", edit.Int(3)), setAt("vb.k", edit.None{})},
		{setAt("h5.x", edit.Int(3)), setAt("h5.ev", edit.Key("two"))},
	} {
		s := open(t, typedUndoFS(), nil, "", "d")
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
		var oe *edit.OpError
		if !errors.As(err, &oe) || oe.Index != 1 || oe.Path != ops[1].Path {
			t.Errorf("%+v: %v, want the refusal named by operation 1 (%s), the driver's whose cascade it is", ops, err, ops[1].Path)
		}
	}
}
