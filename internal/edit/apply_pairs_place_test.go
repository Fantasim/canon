package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// twoSrc has two pairs fields side by side, the first without a default, loaded from files
// writing neither, one or the other, and from a map read past an `at:` `*` (WIRE.md 6.3).
const twoSrc = `/// P.
package d

/// A stat.
record Stat {
  /// Which.
  attr: String
  /// How much.
  val: Int = 0
}

/// Two lists of pairs.
record Two {
  /// A name.
  name: String = ""
  /// First, no default.
  a: [Stat](..=2) @json(pairs: ["a{i}", "x{i}"])
  /// Second.
  b: [Stat](..=2) = [] @json(pairs: ["b{i}", "y{i}"])
  /// A note.
  note: String = ""
}

/// Neither.
let w: Two = load("w.json")

/// The second only.
let p: Two = load("p.json")

/// The first only.
let q: Two = load("q.json")

/// Read past a star.
let s: {String: Two} = load("s.json", at: "*.t")
`

func twoFile(body string) string {
	return "{\n  \"name\": \"n\",\n" + body + "  \"note\": \"z\"\n}\n"
}

const (
	twoA = "  \"a0\": \"q\",\n  \"x0\": 1,\n"
	twoB = "  \"b0\": \"r\",\n  \"y0\": 2,\n"
	twoS = "{\n  \"A\": {\n    \"t\": {\n      \"a0\": \"q\",\n      \"x0\": 1\n    }\n  }\n}\n"
)

func twoFS() mapFS {
	return mapFS{
		"law/project.canon": file(projectCanon), "law/d/d.canon": file(twoSrc), "law/d/w.json": file(twoFile("")),
		"law/d/p.json": file(twoFile(twoB)), "law/d/q.json": file(twoFile(twoA)), "law/d/s.json": file(twoS),
	}
}

// WIRE.md 5.14, 6.3, API.md M3, M4, M8, E22 (log-2026-09-29 M4 B11): new slots go at their field's
// place, beside another pairs field's slots, inside an `at:` `*` item too; a field without a
// default comes back by Set([]); each file is as expected and comes back byte for byte.
func TestPairsPlacement(t *testing.T) {
	st := func(attr string, val int64) edit.Lit { return edit.Obj{"attr": edit.Str(attr), "val": edit.Int(val)} }
	star := func(body string) string { return "{\n  \"A\": {\n    \"t\": {\n" + body + "    }\n  }\n}\n" }
	for _, c := range []struct {
		name, file, want string
		ops              []edit.Operation
	}{
		{"no default, no slot", "law/d/w.json", twoFile(twoA), []edit.Operation{{Kind: edit.OpAdd, Path: "w.a", Value: st("q", 1)}}},
		{"before the other's slots", "law/d/p.json", twoFile(twoA + twoB), []edit.Operation{{Kind: edit.OpAdd, Path: "p.a", Value: st("q", 1)}}},
		{"after the other's slots", "law/d/q.json", twoFile(twoA + twoB), []edit.Operation{{Kind: edit.OpAdd, Path: "q.b", Value: st("r", 2)}}},
		{"remove beside the other's", "law/d/q.json", twoFile(""), []edit.Operation{{Kind: edit.OpRemove, Path: "q.a[0]"}}},
		{"star, add", "law/d/s.json", star("      \"a0\": \"q\",\n      \"x0\": 1,\n      \"a1\": \"v\",\n      \"x1\": 4\n"),
			[]edit.Operation{{Kind: edit.OpAdd, Path: `s["A"].a`, Value: st("v", 4)}}},
		{"star, insert and move", "law/d/s.json", star("      \"a0\": \"q\",\n      \"x0\": 1,\n      \"a1\": \"v\",\n      \"x1\": 4\n"),
			[]edit.Operation{{Kind: edit.OpInsert, Path: `s["A"].a`, Index: 0, Value: st("v", 4)}, {Kind: edit.OpMove, Path: `s["A"].a[0]`, Index: 1}}},
		{"star, remove", "law/d/s.json", "{\n  \"A\": {\n    \"t\": {}\n  }\n}\n", []edit.Operation{{Kind: edit.OpRemove, Path: `s["A"].a[0]`}}},
		{"star, other field", "law/d/s.json", star("      \"a0\": \"q\",\n      \"x0\": 1,\n      \"b0\": \"r\",\n      \"y0\": 2\n"),
			[]edit.Operation{{Kind: edit.OpAdd, Path: `s["A"].b`, Value: st("r", 2)}}},
	} {
		s := open(t, twoFS(), nil, "", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := string(written(twoFS(), plan)[c.file].Data); got != c.want {
			t.Errorf("%s: %s is\n%s\nwant\n%s", c.name, c.file, got, c.want)
		}
		roundTrip(t, c.name, twoFS(), c.ops)
	}
}

// API.md E23 (log-2026-09-29 M4 B11): the Undo of an Add to a pairs field without a default and
// without a slot is Set of the empty list, not Reset.
func TestPairsUndoNoDefault(t *testing.T) {
	s := open(t, twoFS(), nil, "", "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{{Kind: edit.OpAdd, Path: "w.a", Value: stat("q", 1)}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Undo) != 1 || plan.Undo[0].Kind != edit.OpSet || plan.Undo[0].Path != "d:w.a" {
		t.Errorf("Undo %+v, want Set of d:w.a", plan.Undo)
	}
}

// API.md V1, V2, WIRE.md 5.14 (log-2026-09-29 M4 B11): a pairs list past its slots has no wire
// form, so Add, Insert and a whole Set of it, or of a record holding it, going into a JSON source
// are a ValueError and write nothing; stated in Canon it is left to the re-check.
func TestPairsPastSlots(t *testing.T) {
	four := edit.List{stat("a", 1), stat("b", 2), stat("c", 3), stat("d", 4)}
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"add", []edit.Operation{{Kind: edit.OpAdd, Path: "t.stats", Value: stat("w", 3)}, {Kind: edit.OpAdd, Path: "t.stats", Value: stat("v", 4)}}},
		{"insert", []edit.Operation{{Kind: edit.OpInsert, Path: "t.stats", Value: stat("w", 3)}, {Kind: edit.OpInsert, Path: "t.stats", Index: 1, Value: stat("v", 4)}}},
		{"set the list", []edit.Operation{{Kind: edit.OpSet, Path: "t.stats", Value: four}}},
		{"set into an absent field", []edit.Operation{{Kind: edit.OpSet, Path: "u.stats", Value: four}}},
		{"set the record", []edit.Operation{{Kind: edit.OpSet, Path: "h.thing", Value: edit.Obj{"stats": four}}}},
	} {
		s := open(t, pairsFS(), nil, "", "d")
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		var ve *edit.ValueError
		if !errors.As(err, &ve) || !errors.Is(err, edit.ErrBadValue) {
			t.Errorf("%s: %v, want a ValueError", c.name, err)
		}
	}
	s := open(t, pairsFS(), nil, "", "d")
	if _, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{{Kind: edit.OpSet, Path: "c.stats", Value: four}}}); err != nil {
		t.Errorf("canon: %v, want the value left to the re-check", err)
	}
}
