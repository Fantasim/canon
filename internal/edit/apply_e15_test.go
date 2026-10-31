package edit_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
)

// valuedSrc's maps have dependent values, not dependent keys: one required with a default, one
// optional, both holding values only the kill branch takes.
const valuedSrc = `package d

local enum Goal { kill, paint }

local enum Color { red, green }

local type P(g: Goal) = match g {
  kill => Int
  paint => Color
}

local record R {
  goal: Goal = kill
  vals: {String: P(goal)} = {}
  opt: {String: P(goal)}?
}

local let r: R = { vals: { "x": 1 }, opt: { "y": 2 } }
`

// API.md E15 (DECISIONS 411): only a finding at a field's exact path drops it; a map with
// dependent values, required or optional, keeps its value with its E3802 at the entry.
func TestE15KeepsDependentValuedMap(t *testing.T) {
	fsys := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(valuedSrc)}
	s := open(t, fsys, nil, "", "d")
	if n := s.a.Result().Summary.Errors; n > 0 {
		t.Fatalf("valuedSrc has %d errors: %v", n, s.a.Result().List)
	}
	op := edit.Operation{Kind: edit.OpSet, Path: "d:r.goal", Value: edit.Member("paint")}
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Dropped) != 0 {
		t.Errorf("dropped %+v, want none", plan.Dropped)
	}
	after := written(fsys, plan)
	if text := string(after["law/d/d.canon"].Data); !strings.Contains(text, `vals: { "x": 1 }, opt: { "y": 2 }`) {
		t.Errorf("after the edit:\n%s", text)
	}
	var at []string
	for _, f := range open(t, after, nil, "", "d").a.Result().List {
		if f.Code == diag.E3802.Def().Code {
			at = append(at, f.Path)
		}
	}
	if !slices.Equal(at, []string{"r.vals[x]", "r.opt[y]"}) {
		t.Errorf("type mismatches at %v, want [r.vals[x] r.opt[y]]", at)
	}
}

// byPickFS is depSrc's project whose tag counts one pick of its default goal's branch.
func byPickFS() mapFS {
	return depFSWith(strings.Replace(depRoot, `  "tag": {
    "pick": "red"
  },`, `  "tag": {
    "pick": "red",
    "byPick": {
      "red": 1
    }
  },`, 1))
}

// API.md E15, E22, E23 (log-2026-09-29 M4 B7-r, B7-r4, B10): after the Undo, every file comes back
// byte for byte: a ref discriminant, a dependent-keyed map dropped to its default (JSON, computed
// or not, and Canon), an Add or AddEntry that wrote its defaulted collection.
func TestUndoCascades(t *testing.T) {
	for _, c := range []struct {
		name string
		fsys func() mapFS
		ops  []edit.Operation
	}{
		{"ref discriminant", depFS, []edit.Operation{{Kind: edit.OpSet, Path: "objectives.o1.def", Value: edit.Key("art")}}},
		{"keyed map, Never", byPickFS, []edit.Operation{{Kind: edit.OpSet, Path: "root.tag.goal", Value: edit.Member("visit")}}},
		{"keyed map, not decoded", undoFS, []edit.Operation{{Kind: edit.OpSet, Path: "root.tag.goal", Value: edit.Member("paint")}}},
		{"held value kept, ref branch", undoFS, []edit.Operation{{Kind: edit.OpSet, Path: "root.tag.goal", Value: edit.Member("kill")}}},
		{"keyed map, Canon", depFS, []edit.Operation{{Kind: edit.OpSet, Path: "canonTag.goal", Value: edit.Member("visit")}}},
		{"AddEntry, defaulted map", depFS, []edit.Operation{{Kind: edit.OpAddEntry, Path: "root.tag.byPick", Key: edit.Member("green"), Value: edit.Int(1)}}},
		{"AddEntry twice, defaulted map", depFS, []edit.Operation{
			{Kind: edit.OpAddEntry, Path: "root.tag.byPick", Key: edit.Member("green"), Value: edit.Int(1)},
			{Kind: edit.OpAddEntry, Path: "root.tag.byPick", Key: edit.Member("red"), Value: edit.Int(2)},
		}},
		{"Add, defaulted list", depFS, []edit.Operation{{Kind: edit.OpAdd, Path: "root.steps", Value: edit.Source("{ name: \"s\", act: stay }")}}},
		{"Insert, defaulted list", depFS, []edit.Operation{{Kind: edit.OpInsert, Path: "root.steps", Value: edit.Source("{ name: \"s\", act: stay }")}}},
	} {
		roundTrip(t, c.name, c.fsys(), c.ops)
	}
}

// API.md E15 (log-2026-09-29 M4 B7-r4): a map keyed by a dependent type whose keys fit the new
// branch no longer is dropped to its default and reported; one whose keys still fit is kept, a
// key naming no entry of a ref being the re-check's E3501, not a mismatch.
func TestDropKeyedMap(t *testing.T) {
	for _, c := range []struct {
		name, path string
		goal       string
		dropped    []string
	}{
		{"JSON, Never", "root.tag.goal", "visit", []string{"d:root.tag.byPick", "d:root.tag.pick"}},
		{"JSON, ref", "root.tag.goal", "kill", nil},
		{"Canon, Never", "canonTag.goal", "visit", []string{"d:canonTag.byPick"}},
	} {
		s := open(t, byPickFS(), nil, "", "d")
		op := edit.Operation{Kind: edit.OpSet, Path: c.path, Value: edit.Member(c.goal)}
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var got []string
		for _, d := range plan.Dropped {
			got = append(got, d.Path)
		}
		if strings.Join(got, " ") != strings.Join(c.dropped, " ") {
			t.Errorf("%s: dropped %v, want %v", c.name, got, c.dropped)
		}
	}
}
