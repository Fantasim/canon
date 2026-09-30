package edit_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// regionTexts are the regions of every write of plan's first change, as op:kind:the text of
// the file before them that they cover.
func regionTexts(plan *edit.Plan) []string {
	var out []string
	for _, w := range edit.Writes(plan, plan.Changes[0].Path) {
		for _, r := range w.Regions {
			out = append(out, fmt.Sprintf("%d:%d:%s", w.Op, r.Kind, w.Before[r.Lo:r.Hi]))
		}
	}
	return out
}

// API.md V1, V3, WIRE.md 5.14 (log-2026-09-29 M4 B11-r): a pair missing a field with no default
// leaves a lone slot key (E7117), so going into a JSON source it is a ValueError in every form;
// stated in Canon it is left to the re-check (E3302).
func TestPairsMissingField(t *testing.T) {
	miss := edit.Obj{"val": edit.Int(3)}
	for _, c := range []struct {
		name string
		op   edit.Operation
	}{
		{"add", edit.Operation{Kind: edit.OpAdd, Path: "t.stats", Value: miss}},
		{"set an element", edit.Operation{Kind: edit.OpSet, Path: "t.stats[0]", Value: miss}},
		{"set the list", edit.Operation{Kind: edit.OpSet, Path: "t.stats", Value: edit.List{miss}}},
		{"add into an absent field", edit.Operation{Kind: edit.OpAdd, Path: "u.stats", Value: miss}},
		{"set a record, parent absent", edit.Operation{Kind: edit.OpSet, Path: "h.thing", Value: edit.Obj{"stats": edit.List{miss}}}},
	} {
		s := open(t, pairsFS(), nil, "", "d")
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		var ve *edit.ValueError
		if !errors.As(err, &ve) || !strings.Contains(ve.Detail, "attr") {
			t.Errorf("%s: %v, want a ValueError naming attr", c.name, err)
		}
	}
	s := open(t, pairsFS(), nil, "", "d")
	if _, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{{Kind: edit.OpAdd, Path: "c.stats", Value: miss}}}); err != nil {
		t.Errorf("canon: %v, want it left to the re-check", err)
	}
}

// numSrc has pairs whose values are numbers a file may write in more than one text.
const numSrc = `/// P.
package d

/// A float stat.
record FS {
  /// Which.
  n: String
  /// How much.
  x: Float
}

/// A wait.
record DS {
  /// Which.
  n: String
  /// How long.
  d: Duration @json(unit: s)
}

/// A thing.
record Thing {
  /// Floats.
  fs: [FS](..=3) = [] @json(pairs: ["fn{i}", "fx{i}"])
  /// Waits.
  ds: [DS](..=3) = [] @json(pairs: ["dn{i}", "dd{i}"])
}

/// Loaded.
let t: Thing = load("t.json")
`

// numCanon writes its numbers as `canon fmt --json-sources` does; numLoose writes the same values
// otherwise (1.50, 1e0, 5.0 for 5 s).
const (
	numCanon = "{\n  \"fn0\": \"a\",\n  \"fx0\": 1.5,\n  \"fn1\": \"b\",\n  \"fx1\": 1,\n  \"dn0\": \"w\",\n  \"dd0\": 5\n}\n"
	numLoose = "{\n  \"fn0\": \"a\",\n  \"fx0\": 1.50,\n  \"fn1\": \"b\",\n  \"fx1\": 1e0,\n  \"dn0\": \"w\",\n  \"dd0\": 5.0\n}\n"
)

func numFS(json string) mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(numSrc), "law/d/t.json": file(json)}
}

// API.md M1, M6, E22, WIRE.md 5.14 (log-2026-09-29 M4 B11-r3): on a source M9 holds canonical, an
// operation on floats and unit-s waits writes only the members it adds or changes, and the Undo
// restores every byte (comparing by value, not text, changes nothing a test can see here).
func TestPairsSlotValues(t *testing.T) {
	for _, c := range []struct {
		name, want string
		op         edit.Operation
		writes     []string
	}{
		{"add a float", "{\n  \"fn0\": \"a\",\n  \"fx0\": 1.5,\n  \"fn1\": \"b\",\n  \"fx1\": 1,\n  \"fn2\": \"c\",\n  \"fx2\": 3,\n  \"dn0\": \"w\",\n  \"dd0\": 5\n}\n",
			edit.Operation{Kind: edit.OpAdd, Path: "t.fs", Value: edit.Obj{"n": edit.Str("c"), "x": edit.Float(3)}}, []string{"0:3:", "0:3:"}},
		{"rename a float's pair", "{\n  \"fn0\": \"z\",\n  \"fx0\": 1.5,\n  \"fn1\": \"b\",\n  \"fx1\": 1,\n  \"dn0\": \"w\",\n  \"dd0\": 5\n}\n",
			edit.Operation{Kind: edit.OpSet, Path: "t.fs[0]", Value: edit.Obj{"n": edit.Str("z"), "x": edit.Float(1.5)}}, []string{"0:1:\"a\""}},
		{"add a wait", "{\n  \"fn0\": \"a\",\n  \"fx0\": 1.5,\n  \"fn1\": \"b\",\n  \"fx1\": 1,\n  \"dn0\": \"w\",\n  \"dd0\": 5,\n  \"dn1\": \"v\",\n  \"dd1\": 2\n}\n",
			edit.Operation{Kind: edit.OpAdd, Path: "t.ds", Value: edit.Obj{"n": edit.Str("v"), "d": edit.Source("2s")}}, []string{"0:3:", "0:3:"}},
	} {
		s := open(t, numFS(numCanon), nil, "", "d")
		ops := []edit.Operation{c.op}
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := string(written(numFS(numCanon), plan)["law/d/t.json"].Data); got != c.want {
			t.Errorf("%s: t.json is\n%s\nwant\n%s", c.name, got, c.want)
		}
		if got := regionTexts(plan); !slices.Equal(got, c.writes) {
			t.Errorf("%s: writes %q, want %q", c.name, got, c.writes)
		}
		roundTrip(t, c.name, numFS(numCanon), ops)
	}
}

// API.md M1-M4, M6 (log-2026-09-29 M4 B11-r): only new, changed or removed slot members are
// written: an Add inserts two members, a Remove rewrites the slot values that moved and drops
// the last slot, a multi-op writes only what each op changes.
func TestPairsMinimalWrites(t *testing.T) {
	for _, c := range []struct {
		name string
		ops  []edit.Operation
		want []string
	}{
		{"add", []edit.Operation{{Kind: edit.OpAdd, Path: "t.stats", Value: stat("w", 3)}}, []string{"0:3:", "0:3:"}},
		{"remove", []edit.Operation{{Kind: edit.OpRemove, Path: "t.stats[0]"}},
			[]string{"0:2:\"v1\": 2,\n  ", "0:2:\"k1\": \"y\",\n  ", "0:1:1", "0:1:\"x\""}},
		{"multi-op", []edit.Operation{
			{Kind: edit.OpAdd, Path: "t.stats", Value: stat("w", 3)},
			{Kind: edit.OpMove, Path: "t.stats[0]", Index: 2},
			{Kind: edit.OpSet, Path: "t.stats[2].val", Value: edit.Int(5)},
		}, []string{"0:3:", "0:3:", "1:1:3", "1:1:\"w\"", "1:1:2", "1:1:\"y\"", "1:1:1", "1:1:\"x\"", "2:1:1"}},
	} {
		s := open(t, pairsFS(), nil, "", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := regionTexts(plan); !slices.Equal(got, c.want) {
			t.Errorf("%s: writes %q, want %q", c.name, got, c.want)
		}
	}
}

// WIRE.md 5.14, API.md W8, M7 (log-2026-09-29 M4 B11-r): a record written whole writes each pair
// with both keys, a field left to its default by its value.
func TestPairsWholeRecordDefault(t *testing.T) {
	ops := []edit.Operation{{Kind: edit.OpSet, Path: "h.thing", Value: edit.Obj{"stats": edit.List{edit.Obj{"attr": edit.Str("w")}}}}}
	s := open(t, pairsFS(), nil, "", "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"thing\": {\n    \"k0\": \"w\",\n    \"v0\": 0\n  },\n  \"tail\": \"z\"\n}\n"
	if got := string(written(pairsFS(), plan)["law/d/h.json"].Data); got != want {
		t.Errorf("h.json is\n%s\nwant\n%s", got, want)
	}
}

// API.md M1, M6, E22, WIRE.md 5.8 (log-2026-09-29 M4 B11-r): a map holding held symbol keys is
// compared key by key, so a Set changing one value writes that value alone.
func TestHeldKeyMapMinimal(t *testing.T) {
	json := strings.Replace(itemCarryJSON, "\"a\": {\n      \"zzz\": 5\n    }", "\"a\": {\n      \"zzz\": 5,\n      \"yyy\": 6\n    }", 1)
	fs := func() mapFS {
		return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(itemCarrySrc), "law/d/t.json": file(json)}
	}
	ops := []edit.Operation{{Kind: edit.OpSet, Path: `tag.waits["a"]`, Value: edit.FromJSON(`{"zzz": 5, "yyy": 7}`)}}
	s := open(t, fs(), nil, "", "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := regionTexts(plan), []string{"0:1:6"}; !slices.Equal(got, want) {
		t.Errorf("writes %q, want %q", got, want)
	}
	roundTrip(t, "held keys", fs(), ops)
}
