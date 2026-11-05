package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// heldSrc's fields have types their record's driver computes: an enum (slot), or the entry an
// optional ref names (pick, bag, form, nest: its first branch a type function); other, loaded,
// holds the branch of b, not the first.
const heldSrc = `package d

local enum Kind { a, b }

local type Slot(k: Kind) = match k {
  a => [Int]
  b => [String]
}

local record Ev {
  kind: Kind
}

local let evs: table Ev = { one { kind: a }, two { kind: b } }

local record Item {
  n: Int
}

local variant Shape {
  dot { n: Int }
  ring
}

local type Pick(e: Ev) = match e.kind {
  a => [Item]
  b => [String]
}

local type Bag(e: Ev) = match e.kind {
  a => {String: [Int]}
  b => {String: [String]}
}

local type Wrap(e: Ev) = match e.kind {
  a => [String]
  b => [String]
}

local type Nest(e: Ev) = match e.kind {
  a => Wrap(e)
  b => [Item]
}

local type Form(e: Ev) = match e.kind {
  a => Shape
  b => String
}

local record Held {
  kind: Kind = a
  slot: Slot(kind)
  ev: ref evs?
  pick: Pick(ev)
  bag: Bag(ev)
  form: Form(ev)
  nest: Nest(ev)
}

local let held: Held = { slot: [1], ev: one, pick: [{ n: 1 }], bag: { "x": [1] }, form: ring, nest: ["s"] }

local let other: Held = load("other.json")
`

// otherJSON is other: its ev's kind b selects every field's second branch.
const otherJSON = `{"slot": [1], "ev": "two", "pick": ["s"], "bag": {"x": ["s"]}, "form": "s", "nest": [{"n": 1}]}
`

// noEv is the op that makes root's dependent fields compute Never: their optional argument reads
// none (TYPES.md 11.6, DECISIONS 307).
func noEv(root string) edit.Operation {
	return edit.Operation{Kind: edit.OpSet, Path: "d:" + root + ".ev", Value: edit.None{}}
}

// noEntry is a Set of root whose ev names no entry, so the typer computes no branch for its
// dependent fields: field is given value, the others a value of their first branch.
func noEntry(root, field, value string) edit.Operation {
	given := map[string]string{"pick": `[{ n: 1 }]`, "bag": `{ "x": [1] }`, "form": `"s"`, "nest": `["s"]`}
	given[field] = value
	text := `{ ev: nope, slot: [1]`
	for _, f := range []string{"pick", "bag", "form", "nest"} {
		text += ", " + f + ": " + given[f]
	}
	return edit.Operation{Kind: edit.OpSet, Path: "d:" + root, Value: edit.Source(text + " }")}
}

// noEntryObj is noEntry as an Obj: field is given v, the others the Source noEntry gives them,
// which their first branch takes, so only v can be refused.
func noEntryObj(root, field string, v edit.Lit) edit.Operation {
	obj := edit.Obj{"ev": edit.Key("nope"), "slot": edit.Source(`[1]`), "pick": edit.Source(`[{ n: 1 }]`),
		"bag": edit.Source(`{ "x": [1] }`), "form": edit.Source(`"s"`), "nest": edit.Source(`["s"]`)}
	obj[field] = v
	return edit.Operation{Kind: edit.OpSet, Path: "d:" + root, Value: obj}
}

func setOf(path string, v edit.Lit) edit.Operation {
	return edit.Operation{Kind: edit.OpSet, Path: path, Value: v}
}

// API.md V1 (DECISIONS 411, 171, 307): a Set's list, record, map or case literal given to a
// dependent type takes the branch the earlier ops left (in Canon text only its form's first, round
// 5); none computed, a *ValueError, but Canon text takes its first branch; Never takes none.
func TestDependentCollectionLiteralUncomputed(t *testing.T) {
	s := open(t, mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(heldSrc), "law/d/other.json": file(otherJSON)}, nil, "", "d")
	if n := s.a.Result().Summary.Errors; n > 0 {
		t.Fatalf("heldSrc has %d errors: %v", n, s.a.Result().List)
	}
	for _, c := range []struct {
		name    string
		ops     []edit.Operation
		refused bool
	}{
		{"computed", []edit.Operation{setOf("d:held.kind", edit.Member("b")), setOf("d:held.kind", edit.Member("a")), setOf("d:held.slot", edit.List{edit.Int(3)})}, false},
		{"computed, Canon, not the first list branch", []edit.Operation{setOf("d:held.kind", edit.Member("b")), setOf("d:held.slot", edit.List{edit.Str("x")})}, true},
		{"computed, Canon, the first list branch", []edit.Operation{setOf("d:held.slot", edit.List{edit.Int(3)})}, false},
		{"list", []edit.Operation{noEntryObj("held", "pick", edit.List{edit.Obj{"n": edit.Int(3)}})}, true},
		{"list, Never", []edit.Operation{noEv("held"), setOf("d:held.pick", edit.List{edit.Obj{"n": edit.Int(3)}})}, true},
		{"case", []edit.Operation{noEntryObj("held", "form", edit.Case{Name: "dot", Fields: edit.Obj{"n": edit.Int(2)}})}, true},
		{"case, Never", []edit.Operation{noEv("held"), setOf("d:held.form", edit.Case{Name: "dot", Fields: edit.Obj{"n": edit.Int(2)}})}, true},
		{"Obj, others Source", []edit.Operation{noEntryObj("held", "form", edit.Source(`"t"`))}, false},
		{"source, first branch", []edit.Operation{noEntry("held", "pick", `[{ n: 2 }]`)}, false},
		{"source, JSON, first branch", []edit.Operation{noEntry("other", "pick", `[{ n: 2 }]`)}, true},
		{"source, Never", []edit.Operation{noEv("other"), setOf("d:other.pick", edit.Source(`[{ n: 2 }]`))}, true},
		{"source, not the first branch", []edit.Operation{noEntry("held", "pick", `["t"]`)}, true},
		{"source, nested first branch", []edit.Operation{noEntry("held", "nest", `["t"]`)}, false},
		{"source, nested, Never", []edit.Operation{noEv("held"), setOf("d:held.nest", edit.Source(`["t"]`))}, true},
		{"source, not the nested first branch", []edit.Operation{noEntry("held", "nest", `[{ n: 2 }]`)}, true},
	} {
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		var ve *edit.ValueError
		if errors.As(err, &ve) != c.refused || !c.refused && err != nil {
			t.Errorf("%s: %v, want a *ValueError: %v", c.name, err, c.refused)
		}
	}
}

// API.md E1, V1 (DECISIONS 307): a container whose optional argument reads none computes Never,
// which holds no item: an Insert or AddEntry into it is ErrBadOp.
func TestDependentItemIntoNever(t *testing.T) {
	s := open(t, mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(heldSrc), "law/d/other.json": file(otherJSON)}, nil, "", "d")
	for _, op := range []edit.Operation{
		{Kind: edit.OpInsert, Path: "d:held.pick", Index: 0, Value: edit.Obj{"n": edit.Int(2)}},
		{Kind: edit.OpAddEntry, Path: "d:held.bag", Key: edit.Key("y"), Value: edit.List{edit.Int(2)}},
	} {
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{noEv("held"), op}})
		if !errors.Is(err, edit.ErrBadOp) {
			t.Errorf("%s: %v, want ErrBadOp", op.Path, err)
		}
	}
}
