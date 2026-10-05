package edit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// undoRoot is heldRoot with more data verification refuses and the edit keeps as written
// (DECISIONS 175): a Never value that is no string, a Never map value and key, a dependent map's
// Never entry.
var undoRoot = strings.NewReplacer(`      "def": "tour",
      "target": "anywhere"
    }
  ],`, `      "def": "tour",
      "target": 5,
      "named": {
        "a": {
          "x": 1
        }
      }
    }
  ],`, `  "aims": {
    "hunt": "wolf"
  },`, `  "aims": {
    "hunt": "wolf",
    "tour": "x"
  },`, `  "tag": {
    "pick": "red"
  },`, `  "tag": {
    "goal": "visit",
    "pick": "p",
    "byPick": {
      "zzz": 1
    }
  },`).Replace(heldRoot)

// undoObjs is depObjs with an entry holding a Never symbol.
const undoObjs = `{
  "o1": {
    "def": "hunt",
    "target": "wolf"
  },
  "o2": {
    "def": "tour",
    "target": "anywhere"
  }
}
`

// paramSrc is a package whose dependent field reads its discriminant from a default computed
// from the record's parameter (TYP-15, TYPES.md 11.1).
const paramSrc = `/// P.
package d

/// A goal.
enum Goal { kill, paint }

/// A colour.
enum Color { red, green = "GREEN" }

/// A goal definition.
record GoalDef {
  /// Its goal.
  goal: Goal = paint
}

/// The goal definitions.
let goalDefs: table GoalDef = { hunt { goal: kill }, art {} }

/// What a goal picks.
type Pick(g: Goal) = match g {
  kill => String
  paint => Color
}

/// A pick through its parameter's default.
record ByParam(d: GoalDef) {
  /// Its goal, from the parameter.
  goal: Goal = d.goal
  /// What it picks.
  pick: Pick(goal)?
}

/// The root.
record Root {
  /// By parameter.
  bp: {d in goalDefs: ByParam(d)} = {}
}

/// Loaded.
let root: Root = load("root.json")
`

const paramRoot = `{
  "bp": {
    "art": {
      "pick": "red"
    }
  }
}
`

func undoFS() mapFS {
	f := depFSWith(undoRoot)
	f["law/d/objs.json"] = file(undoObjs)
	return f
}

// visitFS is depSrc's project whose tag's plain discriminant selects the Never arm, its pick the
// symbol the decoder reads from "p" (DECISIONS 175).
func visitFS() mapFS {
	return depFSWith(strings.Replace(depRoot, `  "tag": {
    "pick": "red"
  },`, `  "tag": {
    "goal": "visit",
    "pick": "p"
  },`, 1))
}

func paramFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(paramSrc), "law/d/root.json": file(paramRoot)}
}

// API.md E22, E23, M6 (log-2026-09-29 M4 B7-r3): every operation kind on JSON sources holding
// data verification refuses gives the original bytes back after its Undo, each write minimal.
func TestUndoDependentData(t *testing.T) {
	src := func(s string) edit.Lit { return edit.Source(s) }
	for _, c := range []struct {
		name string
		fsys func() mapFS
		ops  []edit.Operation
	}{
		{"Set field", undoFS, []edit.Operation{{Kind: edit.OpSet, Path: "root.objs[1].target", Value: edit.None{}}}},
		{"Set record", undoFS, []edit.Operation{{Kind: edit.OpSet, Path: "objectives.o2", Value: src("{ def: art, target: red }")}}},
		{"Reset", undoFS, []edit.Operation{{Kind: edit.OpReset, Path: "root.objs[1].named"}}},
		{"Remove, list", undoFS, []edit.Operation{{Kind: edit.OpRemove, Path: "root.objs[1]"}}},
		{"Remove, map", undoFS, []edit.Operation{{Kind: edit.OpRemove, Path: `root.objs[1].named["a"]`}}},
		{"Remove, dependent map", undoFS, []edit.Operation{{Kind: edit.OpRemove, Path: "root.aims[tour]"}}},
		{"Remove, dependent key", undoFS, []edit.Operation{{Kind: edit.OpRemove, Path: "root.tag.byPick[zzz]"}}},
		{"Add", undoFS, []edit.Operation{{Kind: edit.OpAdd, Path: "root.objs", Value: src("{ def: art, target: green }")}}},
		{"Insert", undoFS, []edit.Operation{{Kind: edit.OpInsert, Path: "root.objs", Index: 0, Value: edit.FromJSON(`{"def":"tour","target":[1]}`)}}},
		{"AddEntry, map", undoFS, []edit.Operation{{Kind: edit.OpAddEntry, Path: "root.objs[1].named", Key: edit.Key("b"), Value: edit.FromJSON(`true`)}}},
		{"AddEntry, dependent map", undoFS, []edit.Operation{{Kind: edit.OpAddEntry, Path: "root.aims", Key: edit.Key("art"), Value: src("green")}}},
		{"Move", undoFS, []edit.Operation{{Kind: edit.OpMove, Path: "root.objs[1]", Index: 0}}},
		{"Rename", undoFS, []edit.Operation{{Kind: edit.OpRename, Path: "objectives.o2", Key: edit.Key("o9")}}},
		{"SetCase, same case", undoFS, []edit.Operation{{Kind: edit.OpSetCase, Path: "root.steps[0].act", Case: "go", Value: edit.Obj{"def": edit.Key("art")}}}},
		{"SetCase, other case", undoFS, []edit.Operation{{Kind: edit.OpSetCase, Path: "root.steps[0].act", Case: "stay"}}},
		{"multi-op", undoFS, []edit.Operation{
			{Kind: edit.OpSet, Path: "objectives.o2.note", Value: edit.Str("n")},
			{Kind: edit.OpSet, Path: "objectives.o2.def", Value: edit.Key("art")},
		}},
		{"parameter default", paramFS, []edit.Operation{{Kind: edit.OpSet, Path: "root.bp[art]", Value: src("{ pick: green }")}}},
		{"Set field, plain discriminant", visitFS, []edit.Operation{{Kind: edit.OpSet, Path: "root.tag.pick", Value: edit.None{}}}},
		{"Set field FromJSON, plain discriminant", visitFS, []edit.Operation{{Kind: edit.OpSet, Path: "root.tag.pick", Value: edit.FromJSON(`"q"`)}}},
		{"Reset discriminant", visitFS, []edit.Operation{{Kind: edit.OpReset, Path: "root.tag.goal"}}},
		{"multi-op, plain discriminant", visitFS, []edit.Operation{
			{Kind: edit.OpSet, Path: "root.tag", Value: src("{ goal: visit, pick: p }")},
			{Kind: edit.OpReset, Path: "root.tag.goal"},
		}},
	} {
		roundTrip(t, c.name, c.fsys(), c.ops)
	}
}

// roundTrip applies ops to fsys, then their Undo, and checks every write (M6) and that each file
// comes back byte for byte (E22).
func roundTrip(t *testing.T, name string, fsys mapFS, ops []edit.Operation) {
	t.Helper()
	s := open(t, fsys, nil, "", "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
	if err != nil {
		t.Errorf("%s: %v", name, err)
		return
	}
	after := written(fsys, plan)
	back := open(t, after, nil, "", "d")
	undo, err := edit.Apply(context.Background(), back.env, back.snap, edit.Request{Ops: plan.Undo})
	if err != nil {
		t.Errorf("%s: E22: the Undo %+v: %v", name, plan.Undo, err)
		return
	}
	for _, p := range []*edit.Plan{plan, undo} {
		for _, ch := range p.Changes {
			checkWritten(t, p, ch)
		}
	}
	restored := written(after, undo)
	for f, want := range fsys { //canon:unordered each file is compared alone
		if string(restored[f].Data) != string(want.Data) {
			t.Errorf("%s: E22: %s does not come back:\n%s", name, f, restored[f].Data)
		}
	}
}
