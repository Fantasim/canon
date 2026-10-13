package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// bitsLaw loads a `@json(bits)` list from a JSON source, whose bitmask holds each member once.
var bitsLaw = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/p/p.canon": file(`package p

/// A flag.
enum Flag @codes(UInt8) { binds = 1, equipBind = 2, rare = 4 }

/// An item.
record Item {
  /// Its id.
  id: String
  /// Its flags.
  flags: [Flag] where it.isUnique() = [] @json(bits)
}

/// Items.
let items: [Item] keyed by id = load("items.json")
`),
	"law/p/items.json": file("[\n  {\n    \"id\": \"sword\",\n    \"flags\": 3\n  }\n]\n"),
}

// wireCase is one operation on a JSON source and the error it gives, nil when it applies.
type wireCase struct {
	op   edit.Operation
	want error
}

// checkWireCases applies each case alone to the project fsys opens, never an ErrInternal.
func checkWireCases(t *testing.T, fsys mapFS, cases []wireCase) {
	t.Helper()
	s := open(t, fsys, nil, "", "p")
	for _, c := range cases {
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		if !errors.Is(err, c.want) || errors.Is(err, edit.ErrInternal) {
			t.Errorf("%v %s: %v, want %v", c.op.Kind, c.op.Path, err, c.want)
		}
	}
}

// API.md V1, WIRE.md 5.3: bound for a JSON source, an operation leaving a bits list holding a
// member twice is ErrBadValue, never ErrInternal, whatever op and form; one that does not applies.
func TestBitsRepeatedRefused(t *testing.T) {
	checkWireCases(t, bitsLaw, []wireCase{
		{edit.Operation{Kind: edit.OpAdd, Path: "items.sword.flags", Value: edit.FromJSON(`"binds"`)}, edit.ErrBadValue},
		{edit.Operation{Kind: edit.OpInsert, Path: "items.sword.flags", Value: edit.Member("equipBind")}, edit.ErrBadValue},
		{edit.Operation{Kind: edit.OpSet, Path: "items.sword.flags", Value: edit.Source("[binds, binds]")}, edit.ErrBadValue},
		{edit.Operation{Kind: edit.OpSet, Path: "items.sword.flags[0]", Value: edit.Source("equipBind")}, edit.ErrBadValue},
		{edit.Operation{Kind: edit.OpSet, Path: "items.sword.flags[1]", Value: edit.Source("binds")}, edit.ErrBadValue},
		{edit.Operation{Kind: edit.OpSet, Path: "items.sword", Value: edit.Source(`{ id: "sword", flags: [rare, rare] }`)}, edit.ErrBadValue},
		{edit.Operation{Kind: edit.OpAdd, Path: "items.sword.flags", Value: edit.Member("rare")}, nil},
		{edit.Operation{Kind: edit.OpSet, Path: "items.sword.flags[1]", Value: edit.Source("rare")}, nil},
	})
}

// noneLaw loads a field whose `@json(none: 0)` marker writes none.
var noneLaw = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/p/p.canon": file(`package p

/// An item.
record Item {
  /// Its id.
  id: String
  /// Its level.
  level: Int? @json(none: 0)
}

/// Items.
let items: [Item] keyed by id = load("items.json")
`),
	"law/p/items.json": file("[\n  {\n    \"id\": \"sword\",\n    \"level\": 3\n  }\n]\n"),
}

// API.md V1, WIRE.md 5.4: bound for a JSON source, a present value its field's none marker writes
// has no wire form: ErrBadValue, never ErrInternal; none itself, and the marker's JSON, apply.
func TestNoneMarkerRefused(t *testing.T) {
	checkWireCases(t, noneLaw, []wireCase{
		{edit.Operation{Kind: edit.OpSet, Path: "items.sword.level", Value: edit.Int(0)}, edit.ErrBadValue},
		{edit.Operation{Kind: edit.OpSet, Path: "items.sword", Value: edit.Source(`{ id: "sword", level: 0 }`)}, edit.ErrBadValue},
		{edit.Operation{Kind: edit.OpAdd, Path: "items", Value: edit.Source(`{ id: "axe", level: 0 }`)}, edit.ErrBadValue},
		{edit.Operation{Kind: edit.OpSet, Path: "items.sword.level", Value: edit.None{}}, nil},
		{edit.Operation{Kind: edit.OpSet, Path: "items.sword.level", Value: edit.FromJSON("0")}, nil},
		{edit.Operation{Kind: edit.OpSet, Path: "items.sword.level", Value: edit.Int(4)}, nil},
	})
}
