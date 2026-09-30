package edit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// DEP-02, API.md V1, TYPES.md 11.5 (log-2026-09-29 M4 B7-r3): a value added to a dependent map is
// read in the frame binding the binder to its new key: a name is written as what it names there,
// a name the branch lacks and a name given a Never branch are refused.
func TestAddEntryDependentMap(t *testing.T) {
	applyDep(t, symCase{"added", edit.Operation{Kind: edit.OpAddEntry, Path: "root.aims", Key: edit.Key("art"), Value: edit.Source("green")}, "d/root.json", []string{`"art": "GREEN"`}})
	applyDep(t, symCase{"applied record", edit.Operation{Kind: edit.OpAddEntry, Path: "root.pers", Key: edit.Key("hunt"), Value: edit.Source("{ aim: bear }")}, "d/root.json", []string{`"aim": "bear"`}})
	s := open(t, depFS(), nil, "", "d")
	for _, op := range []edit.Operation{
		{Kind: edit.OpAddEntry, Path: "root.aims", Key: edit.Key("art"), Value: edit.Source("blue")},
		{Kind: edit.OpAddEntry, Path: "root.aims", Key: edit.Key("tour"), Value: edit.Source("x")},
		{Kind: edit.OpAddEntry, Path: "root.pers", Key: edit.Key("tour"), Value: edit.Source("{ aim: x }")},
	} {
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
		var ve *edit.ValueError
		if !errors.As(err, &ve) {
			t.Errorf("%s %v: %v, want a *ValueError", op.Path, op.Value, err)
		}
	}
}

// TYP-15, DEP-02 (log-2026-09-29 M4 B7-r3): a discriminant defaulted from the record's parameter
// selects the branch as the decoder computes it; E15 keeps the value.
func TestSetDependentParamDefault(t *testing.T) {
	s := open(t, paramFS(), nil, "", "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{{Kind: edit.OpSet, Path: "root.bp[art]", Value: edit.Source("{ pick: green }")}}})
	if err != nil {
		t.Fatal(err)
	}
	checkLines(t, symCase{name: "param", file: "d/root.json", want: []string{`"pick": "GREEN"`}}, plan)
	if len(plan.Dropped) != 0 {
		t.Errorf("E15 dropped %+v", plan.Dropped)
	}
}

// API.md E14, E15 (log-2026-09-29 M4 B7-r3): a symbol a SetCase keeps is not resolved against the
// new branch but judged by E15, which drops it; a symbol an earlier operation restored loses its
// exemption when a later one changes the discriminant; a Set to an equal value changes nothing.
func TestDependentSymbolCascades(t *testing.T) {
	for _, c := range []struct {
		name string
		ops  []edit.Operation
		drop []string
	}{
		{"kept by SetCase", []edit.Operation{{Kind: edit.OpSetCase, Path: "root.steps[0].act", Case: "go", Value: edit.Obj{"def": edit.Key("art")}}}, []string{"d:root.steps[0].act.target"}},
		{"retyped later", []edit.Operation{
			{Kind: edit.OpSet, Path: "root.objs[1]", Value: edit.Source(`{ def: tour, target: anywhere, note: "x" }`)},
			{Kind: edit.OpSet, Path: "root.objs[1].def", Value: edit.Key("art")},
		}, []string{"d:root.objs[1].target"}},
		{"equal value", []edit.Operation{{Kind: edit.OpSet, Path: "root.objs[1].def", Value: edit.Key("tour")}}, nil},
	} {
		s := open(t, depFSWith(heldRoot), nil, "", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		var got []string
		for _, d := range plan.Dropped {
			got = append(got, d.Path)
		}
		if strings.Join(got, ",") != strings.Join(c.drop, ",") {
			t.Errorf("%s: dropped %v, want %v", c.name, got, c.drop)
		}
	}
}

// DECISIONS 175, API.md E15 (log-2026-09-29 M4 B7-r3): a held value that is no string is written
// back, and reported, as the JSON token it was read from.
func TestDroppedRawToken(t *testing.T) {
	s := open(t, undoFS(), nil, "", "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{{Kind: edit.OpSet, Path: "root.objs[1].def", Value: edit.Key("art")}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Dropped) == 0 || string(plan.Dropped[0].Value) != "5" {
		t.Errorf("dropped %+v, want the token 5", plan.Dropped)
	}
}

// API.md E25, E22 (log-2026-09-29 M4 B7-r3): the Undo of a Remove of a dependent key carries it as
// a path key, a symbol matched by its Canon name, then its wire value, in .canon and in JSON.
func TestUndoDependentPathKey(t *testing.T) {
	colorKeys := strings.Replace(depRoot, `  "tag": {
    "pick": "red"
  },`, `  "tag": {
    "pick": "red",
    "byPick": {
      "GREEN": 1,
      "red": 2
    }
  },`, 1)
	for _, path := range []string{"root.tag.byPick[green]", `root.tag.byPick["GREEN"]`, "canonTag.byPick[green]"} {
		roundTrip(t, path, depFSWith(colorKeys), []edit.Operation{{Kind: edit.OpRemove, Path: path}})
	}
	s := open(t, depFSWith(colorKeys), nil, "", "d")
	_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{{Kind: edit.OpAddEntry, Path: "root.tag.byPick", Key: edit.PathKey("GREEN"), Value: edit.Int(3)}}})
	if !errors.Is(err, edit.ErrKeyExists) {
		t.Errorf("a path key matched by its wire value: %v, want ErrKeyExists", err)
	}
}
