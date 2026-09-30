package edit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

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
