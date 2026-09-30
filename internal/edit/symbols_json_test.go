package edit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// depSrc is a package whose JSON sources hold dependent values (DEP-02): a field, a list and a
// map of them in records of a list and of a table, a dependent map, and an applied record.
const depSrc = `/// D.
package d

/// A goal.
enum Goal { kill, count, visit, paint }

/// A colour; green's wire value is not its name.
enum Color { red, green = "GREEN" }

/// A mob.
record Mob {
  /// Hit points.
  hp: Int = 1
}

/// The mobs.
let mobs: table Mob = { wolf {}, bear {} }

/// A goal definition.
record GoalDef {
  /// Its goal.
  goal: Goal
}

/// The goal definitions.
let goalDefs: table GoalDef = { hunt { goal: kill }, art { goal: paint }, tour { goal: visit } }

/// What a goal aims at.
type Aim(d: GoalDef) = match d.goal {
  kill => ref mobs
  count => Int
  visit => Never
  paint => Color
}

/// An aim per definition.
record Per(d: GoalDef) {
  /// Its aim.
  aim: Aim(d)? = none
}

/// An objective.
record Obj {
  /// Its goal definition.
  def: ref goalDefs
  /// What it aims at.
  target: Aim(def)? = none
  /// Other aims.
  alts: [Aim(def)] = []
  /// Named aims.
  named: {String: Aim(def)} = {}
  /// A note.
  note: String = ""
}

/// What a goal picks, straight from the goal.
type Pick(g: Goal) = match g {
  kill => ref mobs
  count => String
  visit => Never
  paint => Color
}

/// A tag whose goal is left to its default.
record Tag {
  /// Its goal.
  goal: Goal = paint
  /// What it picks.
  pick: Pick(goal)? = none
  /// Counts by pick.
  byPick: {Pick(goal): Int} = {}
}

/// A tag counting, by default.
record CountTag {
  /// Its goal.
  goal: Goal = count
  /// What it picks.
  pick: Pick(goal)? = none
}

/// A move, written inline in its step.
variant Act {
  /// Going somewhere.
  go {
    /// Its goal definition.
    def: ref goalDefs
    /// What it aims at.
    target: Aim(def)? = none
  }
  /// Staying.
  stay {
    /// For how long.
    minutes: Int = 0
  }
}

/// A step.
record Step {
  /// Its name.
  name: String
  /// Its move.
  act: Act @json(inline)
}

/// The root.
record Root {
  /// Objectives.
  objs: [Obj] = []
  /// Aims per definition.
  aims: {d in goalDefs: Aim(d)} = {}
  /// Records per definition.
  pers: {d in goalDefs: Per(d)} = {}
  /// A tag.
  tag: Tag = {}
  /// A counting tag.
  ctag: CountTag = {}
  /// Steps.
  steps: [Step] = []
}

/// A tag in Canon.
let canonTag: Tag = { byPick: { green: 1 } }

/// Loaded.
let root: Root = load("root.json")

/// A table loaded.
let objectives: table Obj = load("objs.json")
`

// depRoot and depObjs are depSrc's JSON sources, in canonical layout (FMT-02).
const (
	depRoot = `{
  "objs": [
    {
      "def": "art",
      "target": "GREEN"
    }
  ],
  "aims": {
    "hunt": "wolf"
  },
  "pers": {
    "art": {
      "aim": "red"
    }
  },
  "tag": {
    "pick": "red"
  },
  "ctag": {
    "pick": "word"
  }
}
`
	depObjs = `{
  "o1": {
    "def": "hunt",
    "target": "wolf"
  }
}
`
)

func depFS() mapFS { return depFSWith(depRoot) }

// depFSWith is depSrc's project with root, root.json's text.
func depFSWith(root string) mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(depSrc), "law/d/root.json": file(root), "law/d/objs.json": file(depObjs)}
}

// symCase is an operation giving a JSON source a dependent value by a bare name, and the text
// the written file must hold.
type symCase struct {
	name string
	op   edit.Operation
	file string
	want []string
}

// API.md M8, M6, DEP-02: a bare name given a dependent type in a JSON source is written as the
// wire form of what its record's branch names (a key, an enum's wire value), wherever it lies;
// each write is minimal and decodes again without a finding.
func TestSetJSONDependentSymbolWire(t *testing.T) {
	src := func(s string) edit.Lit { return edit.Source(s) }
	for _, c := range []symCase{
		{"record, ref branch", edit.Operation{Kind: edit.OpSet, Path: "root.objs[0]", Value: src("{ def: hunt, target: bear }")}, "d/root.json", []string{`"def": "hunt"`, `"target": "bear"`}},
		{"record, enum branch", edit.Operation{Kind: edit.OpSet, Path: "root.objs[0]", Value: src("{ def: art, target: green, alts: [red, green], named: { \"a\": green } }")}, "d/root.json", []string{`"target": "GREEN"`, `"red"`, `"GREEN"`, `"a": "GREEN"`}},
		{"list field", edit.Operation{Kind: edit.OpSet, Path: "root.objs[0].alts", Value: src("[green, red]")}, "d/root.json", []string{`"GREEN"`, `"red"`}},
		{"map field", edit.Operation{Kind: edit.OpSet, Path: "root.objs[0].named", Value: src(`{ "b": green }`)}, "d/root.json", []string{`"b": "GREEN"`}},
		{"table entry", edit.Operation{Kind: edit.OpSet, Path: "objectives.o1", Value: src("{ def: art, target: green }")}, "d/objs.json", []string{`"def": "art"`, `"target": "GREEN"`}},
		{"record added to a list", edit.Operation{Kind: edit.OpAdd, Path: "root.objs", Value: src("{ def: art, target: green }")}, "d/root.json", []string{`"target": "GREEN"`}},
		{"name added to a list", edit.Operation{Kind: edit.OpAdd, Path: "root.objs[0].alts", Value: src("green")}, "d/root.json", []string{`"GREEN"`}},
		{"dependent map", edit.Operation{Kind: edit.OpSet, Path: "root.aims", Value: src("{ hunt: wolf, art: green }")}, "d/root.json", []string{`"art": "GREEN"`}},
		{"applied record", edit.Operation{Kind: edit.OpSet, Path: "root.pers", Value: src("{ art: { aim: green } }")}, "d/root.json", []string{`"aim": "GREEN"`}},
	} {
		plan := applyDep(t, c)
		back := open(t, written(depFS(), plan), nil, "", "d")
		if back.a.Result().Summary.Errors > 0 {
			t.Errorf("%s: the written source does not decode: %v", c.name, back.a.Result().List)
		}
	}
}

// API.md V1, as a Set of the field alone types the name (log-2026-09-29 U4a G2): a name the
// branch its record selects does not hold is a *ValueError, and nothing is written.
func TestSetJSONDependentSymbolRefused(t *testing.T) {
	s := open(t, depFS(), nil, "", "d")
	for _, op := range []edit.Operation{
		{Kind: edit.OpSet, Path: "root.objs[0]", Value: edit.Source("{ def: art, target: blue }")},
		{Kind: edit.OpSet, Path: "root.objs[0].named", Value: edit.Source(`{ "b": blue }`)},
		{Kind: edit.OpAdd, Path: "root.objs[0].alts", Value: edit.Source("blue")},
	} {
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
		var ve *edit.ValueError
		if !errors.As(err, &ve) || ve.Got != "blue" {
			t.Errorf("%s: %v, want a *ValueError for blue", op.Path, err)
		}
	}
}

// applyDep applies c's operation to depFS and checks its writes (M6) and c.file's text.
func applyDep(t *testing.T, c symCase) *edit.Plan {
	t.Helper()
	return applyDepOn(t, depFS(), c)
}

// applyDepOn is applyDep on fsys.
func applyDepOn(t *testing.T, fsys mapFS, c symCase) *edit.Plan {
	t.Helper()
	s := open(t, fsys, nil, "", "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
	if err != nil {
		t.Fatalf("%s: %v", c.name, err)
	}
	checkLines(t, c, plan)
	return plan
}

// checkLines checks each change of plan (M6) and that c.file holds c.want.
func checkLines(t *testing.T, c symCase, plan *edit.Plan) {
	t.Helper()
	var after string
	for _, ch := range plan.Changes {
		checkWritten(t, plan, ch)
		if ch.Path == c.file {
			after = string(ch.After)
		}
	}
	for _, w := range c.want {
		if !strings.Contains(after, w) {
			t.Errorf("%s: %s lacks %s:\n%s", c.name, c.file, w, after)
		}
	}
}

// API.md M6, M8, DEP-02 (found by FuzzMinimalWriteAll): heistia's dependent field set by a bare
// word, alone, at none or in its record, writes the key of the branch its event's entry selects.
func TestSetJSONDependentSymbol(t *testing.T) {
	dir, roots := exampleRoots(t)
	fz := openFuzzProject(t, dir, roots)
	const heistia = "@resource/Server/System/heistia_config.json"
	for _, c := range []symCase{
		{"field", edit.Operation{Kind: edit.OpSet, Path: "resource.heistia:heistia.tasks[1].filterParam", Value: edit.Source("II_GEN_MAT_ORICHALCUM01")}, heistia, []string{`"filterParam": "II_GEN_MAT_ORICHALCUM01"`}},
		{"field at none", edit.Operation{Kind: edit.OpSet, Path: "resource.heistia:heistia.tasks[0].filterParam", Value: edit.Source("AIBATT1")}, heistia, []string{`"filterParam": "AIBATT1"`}},
		{"record", edit.Operation{Kind: edit.OpSet, Path: "resource.heistia:heistia.tasks[1]", Value: edit.Source(
			`{ eventType: ECONOMY_DROP_ITEM, filterParam: II_GEN_MAT_ORICHALCUM01, targetPerPlayer: 10, maxDuration: 30m, description: "Drop moonstone" }`)}, heistia, []string{`"filterParam": "II_GEN_MAT_ORICHALCUM01"`}},
		{"dependent keys", edit.Operation{Kind: edit.OpSet, Path: "resource.adventurequest:adventureQuests.hourlyTargets[ECONOMY_DROP_ITEM].ratesBySpecific", Value: edit.Source(
			`{ II_GEN_MAT_MOONSTONE: { Stage_1: 26, Stage_2: 30, Stage_3: 0 } }`)}, "@resource/Server/Quest/adventure_quest_config.json", []string{`"II_GEN_MAT_MOONSTONE": {`, `"Stage_1": 26`}},
	} {
		plan, err := edit.Apply(context.Background(), fz.env, edit.NewSnapshot(fz.a), edit.Request{Ops: []edit.Operation{c.op}})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		checkLines(t, c, plan)
	}
}
