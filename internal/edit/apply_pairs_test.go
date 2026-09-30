package edit_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// pairsSrc has a @json(pairs:) list (WIRE.md 5.14) loaded from a file writing two slots, from
// one writing none, below a parent record its file leaves out, and stated in Canon.
const pairsSrc = `/// P.
package d

/// A mark.
enum Mark @codes(UInt8) { a = 1, b = 2, c = 4 }

/// A stat.
record Stat {
  /// Which.
  attr: String
  /// How much.
  val: Int = 0
}

/// A thing.
record Thing {
  /// A name.
  name: String = ""
  /// Marks.
  marks: [Mark] = [] @json(bits)
  /// Stats.
  stats: [Stat](..=3) = [] @json(pairs: ["k{i}", "v{i}"])
  /// A note.
  note: String = ""
}

/// A holder.
record Holder {
  /// Its thing.
  thing: Thing = {}
  /// A tail.
  tail: String = ""
}

/// Loaded, two slots.
let t: Thing = load("t.json")

/// Loaded, no slot.
let u: Thing = load("u.json")

/// Loaded, no thing.
let h: Holder = load("h.json")

/// In Canon.
let c: Thing = { marks: [a], stats: [{ attr: "x", val: 1 }] }
`

const pairsT = `{
  "name": "n",
  "marks": 3,
  "k0": "x",
  "v0": 1,
  "k1": "y",
  "v1": 2,
  "note": "z"
}
`

func pairsFS() mapFS {
	return mapFS{
		"law/project.canon": file(projectCanon), "law/d/d.canon": file(pairsSrc), "law/d/t.json": file(pairsT),
		"law/d/u.json": file("{\n  \"name\": \"n\",\n  \"note\": \"z\"\n}\n"), "law/d/h.json": file("{\n  \"tail\": \"z\"\n}\n"),
	}
}

func stat(attr string, val int64) edit.Lit {
	return edit.Obj{"attr": edit.Str(attr), "val": edit.Int(val)}
}

// WIRE.md 5.14, API.md M1-M6, E22, X2 (log-2026-09-29 M4 B10-r3): every operation on a pairs list
// rewrites its slot keys in slot order and removes the slots past the new length, as expected;
// its Undo brings every file back byte for byte.
func TestPairsWrittenWhole(t *testing.T) {
	slots := func(body string) string {
		return "{\n  \"name\": \"n\",\n  \"marks\": 3,\n" + body + "  \"note\": \"z\"\n}\n"
	}
	for _, c := range []struct {
		name, file, want string
		ops              []edit.Operation
	}{
		{"add", "law/d/t.json", slots("  \"k0\": \"x\",\n  \"v0\": 1,\n  \"k1\": \"y\",\n  \"v1\": 2,\n  \"k2\": \"w\",\n  \"v2\": 3,\n"),
			[]edit.Operation{{Kind: edit.OpAdd, Path: "t.stats", Value: stat("w", 3)}}},
		{"insert", "law/d/t.json", slots("  \"k0\": \"w\",\n  \"v0\": 3,\n  \"k1\": \"x\",\n  \"v1\": 1,\n  \"k2\": \"y\",\n  \"v2\": 2,\n"),
			[]edit.Operation{{Kind: edit.OpInsert, Path: "t.stats", Index: 0, Value: stat("w", 3)}}},
		{"insert, field left to its default", "law/d/t.json", slots("  \"k0\": \"x\",\n  \"v0\": 1,\n  \"k1\": \"w\",\n  \"v1\": 0,\n  \"k2\": \"y\",\n  \"v2\": 2,\n"),
			[]edit.Operation{{Kind: edit.OpInsert, Path: "t.stats", Index: 1, Value: edit.Obj{"attr": edit.Str("w")}}}},
		{"remove first", "law/d/t.json", slots("  \"k0\": \"y\",\n  \"v0\": 2,\n"),
			[]edit.Operation{{Kind: edit.OpRemove, Path: "t.stats[0]"}}},
		{"remove last", "law/d/t.json", slots("  \"k0\": \"x\",\n  \"v0\": 1,\n"),
			[]edit.Operation{{Kind: edit.OpRemove, Path: "t.stats[1]"}}},
		{"remove all", "law/d/t.json", slots(""),
			[]edit.Operation{{Kind: edit.OpRemove, Path: "t.stats[0]"}, {Kind: edit.OpRemove, Path: "t.stats[0]"}}},
		{"move", "law/d/t.json", slots("  \"k0\": \"y\",\n  \"v0\": 2,\n  \"k1\": \"x\",\n  \"v1\": 1,\n"),
			[]edit.Operation{{Kind: edit.OpMove, Path: "t.stats[1]", Index: 0}}},
		{"add into an absent field", "law/d/u.json", "{\n  \"name\": \"n\",\n  \"k0\": \"w\",\n  \"v0\": 3,\n  \"note\": \"z\"\n}\n",
			[]edit.Operation{{Kind: edit.OpAdd, Path: "u.stats", Value: stat("w", 3)}}},
		{"insert into an absent field", "law/d/u.json", "{\n  \"name\": \"n\",\n  \"k0\": \"w\",\n  \"v0\": 3,\n  \"note\": \"z\"\n}\n",
			[]edit.Operation{{Kind: edit.OpInsert, Path: "u.stats", Index: 0, Value: stat("w", 3)}}},
		{"multi-op", "law/d/t.json", "{\n  \"name\": \"m\",\n  \"marks\": 3,\n  \"k0\": \"y\",\n  \"v0\": 2,\n  \"k1\": \"w\",\n  \"v1\": 3,\n  \"k2\": \"x\",\n  \"v2\": 5,\n  \"note\": \"z\"\n}\n",
			[]edit.Operation{
				{Kind: edit.OpAdd, Path: "t.stats", Value: stat("w", 3)},
				{Kind: edit.OpMove, Path: "t.stats[0]", Index: 2},
				{Kind: edit.OpSet, Path: "t.stats[2].val", Value: edit.Int(5)},
				{Kind: edit.OpSet, Path: "t.name", Value: edit.Str("m")},
			}},
		{"set an element", "law/d/t.json", slots("  \"k0\": \"x\",\n  \"v0\": 1,\n  \"k1\": \"y\",\n  \"v1\": 9,\n"),
			[]edit.Operation{{Kind: edit.OpSet, Path: "t.stats[1]", Value: stat("y", 9)}}},
		{"set the list", "law/d/t.json", slots("  \"k0\": \"q\",\n  \"v0\": 1,\n"),
			[]edit.Operation{{Kind: edit.OpSet, Path: "t.stats", Value: edit.List{stat("q", 1)}}}},
		{"reset the list", "law/d/t.json", slots(""),
			[]edit.Operation{{Kind: edit.OpReset, Path: "t.stats"}}},
		{"reset an element's field", "law/d/t.json", slots("  \"k0\": \"x\",\n  \"v0\": 0,\n  \"k1\": \"y\",\n  \"v1\": 2,\n"),
			[]edit.Operation{{Kind: edit.OpReset, Path: "t.stats[0].val"}}},
	} {
		s := open(t, pairsFS(), nil, "", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := string(written(pairsFS(), plan)[c.file].Data); got != c.want {
			t.Errorf("%s: %s is\n%s\nwant\n%s", c.name, c.file, got, c.want)
		}
		roundTrip(t, c.name, pairsFS(), c.ops)
	}
}

// API.md E23 (log-2026-09-29 M4 B10-r3): an operation on a pairs list is undone by Set of the old
// list, or by Reset of the field when its file wrote no slot, whatever the operation.
func TestPairsUndo(t *testing.T) {
	for _, c := range []struct {
		name string
		op   edit.Operation
		want edit.Op
	}{
		{"add", edit.Operation{Kind: edit.OpAdd, Path: "t.stats", Value: stat("w", 3)}, edit.OpSet},
		{"remove", edit.Operation{Kind: edit.OpRemove, Path: "t.stats[0]"}, edit.OpSet},
		{"move", edit.Operation{Kind: edit.OpMove, Path: "t.stats[0]", Index: 1}, edit.OpSet},
		{"add into an absent field", edit.Operation{Kind: edit.OpAdd, Path: "u.stats", Value: stat("w", 3)}, edit.OpReset},
	} {
		s := open(t, pairsFS(), nil, "", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if len(plan.Undo) != 1 || plan.Undo[0].Kind != c.want || plan.Undo[0].Path != "d:"+c.op.Path[:len("t.stats")] {
			t.Errorf("%s: Undo %+v, want one op %d of the list", c.name, plan.Undo, c.want)
		}
	}
}

// WIRE.md 5.14, API.md M3, M4, E22: a pairs list stated in Canon is edited item by item as any
// list, and comes back byte for byte.
func TestPairsCanon(t *testing.T) {
	for _, ops := range [][]edit.Operation{
		{{Kind: edit.OpAdd, Path: "c.stats", Value: stat("w", 3)}},
		{{Kind: edit.OpInsert, Path: "c.stats", Index: 0, Value: stat("w", 3)}},
		{{Kind: edit.OpRemove, Path: "c.stats[0]"}},
		{{Kind: edit.OpAdd, Path: "c.stats", Value: stat("w", 3)}, {Kind: edit.OpMove, Path: "c.stats[1]", Index: 0}},
	} {
		roundTrip(t, "canon", pairsFS(), ops)
	}
}

// API.md E22, E23 (log-2026-09-29 M4 B7-r4, B10-r2): Add below a parent its file leaves out
// materializes the parent; the Undo, Reset of the list, restores the values and may leave the
// emptied parent as {}.
func TestPairsUndoAbsentParent(t *testing.T) {
	s := open(t, pairsFS(), nil, "", "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{{Kind: edit.OpAdd, Path: "h.thing.stats", Value: stat("w", 3)}}})
	if err != nil {
		t.Fatal(err)
	}
	after := written(pairsFS(), plan)
	if got, want := string(after["law/d/h.json"].Data), "{\n  \"thing\": {\n    \"k0\": \"w\",\n    \"v0\": 3\n  },\n  \"tail\": \"z\"\n}\n"; got != want {
		t.Errorf("W8: h.json is\n%s\nwant\n%s", got, want)
	}
	back := open(t, after, nil, "", "d")
	undo, err := edit.Apply(context.Background(), back.env, back.snap, edit.Request{Ops: plan.Undo})
	if err != nil {
		t.Fatalf("E22: the Undo %+v: %v", plan.Undo, err)
	}
	for _, p := range []*edit.Plan{plan, undo} {
		for _, ch := range p.Changes {
			checkWritten(t, p, ch)
		}
	}
	if got, want := string(written(after, undo)["law/d/h.json"].Data), "{\n  \"thing\": {},\n  \"tail\": \"z\"\n}\n"; got != want {
		t.Errorf("E22: h.json is\n%s\nwant\n%s", got, want)
	}
}
