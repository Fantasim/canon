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

local let held: Held = { slot: [1], ev: one, pick: [{ n: 1 }], bag: { "x": [1] }, form: Shape.dot { n: 1 }, nest: ["s"] }

local let other: Held = load("other.json")
`

// otherJSON is other: its ev's kind b selects every field's second branch.
const otherJSON = `{"slot": [1], "ev": "two", "pick": ["s"], "bag": {"x": ["s"]}, "form": "s", "nest": [{"n": 1}]}
`

// noEv is the op that leaves root's dependent fields with no computable branch.
func noEv(root string) edit.Operation {
	return edit.Operation{Kind: edit.OpSet, Path: "d:" + root + ".ev", Value: edit.None{}}
}

func setOf(path string, v edit.Lit) edit.Operation {
	return edit.Operation{Kind: edit.OpSet, Path: path, Value: v}
}

// API.md V1 (DECISIONS 411, 171): a Set's list, record, map or case literal given to a dependent
// type is typed against the branch the earlier ops left, a *ValueError where none can be computed;
// a Source then takes the first branch. Insert and AddEntry into such a value are as before.
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
		{"computed", []edit.Operation{setOf("d:held.kind", edit.Member("b")), setOf("d:held.slot", edit.List{edit.Str("x")})}, false},
		{"list", []edit.Operation{noEv("held"), setOf("d:held.pick", edit.List{edit.Obj{"n": edit.Int(3)}})}, true},
		{"case", []edit.Operation{noEv("held"), setOf("d:held.form", edit.Case{Name: "dot", Fields: edit.Obj{"n": edit.Int(2)}})}, true},
		{"source, first branch", []edit.Operation{noEv("other"), setOf("d:other.pick", edit.Source(`[{ n: 2 }]`))}, false},
		{"source, not the held branch", []edit.Operation{noEv("other"), setOf("d:other.pick", edit.Source(`["t"]`))}, true},
		{"source, nested first branch", []edit.Operation{noEv("held"), setOf("d:held.nest", edit.Source(`["t"]`))}, false},
		{"source, not the nested first branch", []edit.Operation{noEv("held"), setOf("d:held.nest", edit.Source(`[{ n: 2 }]`))}, true},
		{"insert", []edit.Operation{noEv("held"), {Kind: edit.OpInsert, Path: "d:held.pick", Index: 0, Value: edit.Obj{"n": edit.Int(2)}}}, false},
		{"add entry", []edit.Operation{noEv("held"), {Kind: edit.OpAddEntry, Path: "d:held.bag", Key: edit.Key("y"), Value: edit.List{edit.Int(2)}}}, false},
	} {
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		var ve *edit.ValueError
		if errors.As(err, &ve) != c.refused || !c.refused && err != nil {
			t.Errorf("%s: %v, want a *ValueError: %v", c.name, err, c.refused)
		}
	}
}
