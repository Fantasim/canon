package edit_test

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// questSrc holds fields typed by an earlier field: two of a record, one in a case, a list (SPEC §5.11).
const questSrc = `package d

/// What a quest counts.
enum Goal { kill, collect }

/// What a quest aims at, by goal.
type Target(g: Goal) = match g {
  kill => String(1..)
  collect => Int(1..)
}

/// A reward: gold, or an item.
type Reward(item: Bool) = match item {
  false => Int(1..)
  true => String(1..)
}

/// What a step does.
variant Act {
  /// Hunts.
  hunt {
    /// What it counts.
    goal: Goal
    /// What it aims at.
    target: Target(goal)?
  }
  /// Waits.
  wait {
    /// How long.
    turns: Int = 1
  }
}

/// A quest.
record Quest {
  /// What it counts.
  goal: Goal
  /// What it aims at.
  target: Target(goal)?
  /// A second aim.
  bonus: Target(goal)?
  /// A note.
  note: String = ""
  /// What its first step does.
  act: Act
  /// Whether its rewards are items rather than gold.
  items: Bool = false
  /// What completing it gives.
  rewards: [Reward(items)] = []
}

/// The quests.
let quests: table Quest = {
  slay { goal: kill, target: "wolf", bonus: "bear", act: wait { turns: 2 }, rewards: [100, 50] }
  gather { goal: collect, target: 10, act: hunt { goal: kill, target: "boar" } }
}
`

func questFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(questSrc)}
}

func setAt(path string, v edit.Lit) edit.Operation {
	return edit.Operation{Kind: edit.OpSet, Path: path, Value: v}
}

// API.md E22 over E23's order (log-2026-09-29 U-E22-r), E15, E16: the Undo of a request changing
// a driver and the values it types, or letting a cascade drop one, applies and gives every value
// back; the Undo of that Undo gives the edited values back.
func TestUndoRestoresDriversFirst(t *testing.T) {
	collect, kill := edit.Member("collect"), edit.Member("kill")
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"driver then dependent", []edit.Operation{setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25))}},
		{"two dependents", []edit.Operation{
			setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25)), setAt("quests.slay.bonus", edit.Int(3)),
		}},
		{"dependent set twice", []edit.Operation{
			setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25)), setAt("quests.slay.target", edit.Int(30)),
		}},
		{"dependent, then driver, then cascade", []edit.Operation{setAt("quests.slay.target", edit.Str("boar")), setAt("quests.slay.goal", collect)}},
		{"driver changed twice", []edit.Operation{
			setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25)),
			setAt("quests.slay.goal", kill), setAt("quests.slay.target", edit.Str("boar")),
		}},
		{"cascade and an unrelated field", []edit.Operation{
			setAt("quests.slay.goal", collect), setAt("quests.gather.note", edit.Str("n")), setAt("quests.slay.target", edit.Int(25)),
		}},
		{"an entry added between", []edit.Operation{
			setAt("quests.slay.goal", collect),
			{Kind: edit.OpAddEntry, Path: "quests", Key: edit.Key("hunt"), Value: edit.Source(`{ goal: kill, act: wait { turns: 1 } }`)},
			setAt("quests.slay.target", edit.Int(25)),
		}},
		{"two cascades", []edit.Operation{setAt("quests.slay.goal", collect), setAt("quests.gather.goal", kill)}},
		{"inside a case", []edit.Operation{setAt("quests.gather.act.goal", collect), setAt("quests.gather.act.target", edit.Int(5))}},
		{"list, whole", []edit.Operation{setAt("quests.slay.items", edit.Bool(true)), setAt("quests.slay.rewards", edit.Source(`["potion"]`))}},
		{"list, Remove", []edit.Operation{setAt("quests.slay.items", edit.Bool(true)), {Kind: edit.OpRemove, Path: "quests.slay.rewards[0]"}}},
		{"SetCase, then a Set in the new case", []edit.Operation{
			{Kind: edit.OpSetCase, Path: "quests.slay.act", Case: "hunt", Value: edit.Obj{"goal": collect}},
			setAt("quests.slay.act.target", edit.Int(5)),
		}},
	} {
		undoTwice(t, c.name, questFS(), c.ops)
	}
}

// undoTwice applies ops to fsys, then their Undo, then the Undo of that Undo: the first gives
// back every value of fsys (E22: values, not text, so a literal a write broke may stay broken),
// the second every value ops wrote; every write is minimal (M6).
func undoTwice(t *testing.T, name string, fsys mapFS, ops []edit.Operation) {
	t.Helper()
	undoRounds(t, name, fsys, ops, sameValues)
}

// undoTwiceFiles is undoTwice where every file, in canonical layout, comes back byte for byte too.
func undoTwiceFiles(t *testing.T, name string, fsys mapFS, ops []edit.Operation) {
	t.Helper()
	undoRounds(t, name, fsys, ops, func(t *testing.T, name string, want, got mapFS) {
		t.Helper()
		sameValues(t, name, want, got)
		sameFiles(t, name, want, got)
	})
}

func undoRounds(t *testing.T, name string, fsys mapFS, ops []edit.Operation, same func(*testing.T, string, mapFS, mapFS)) {
	t.Helper()
	edited, ok := applied(t, name, fsys, ops)
	if !ok {
		return
	}
	undone, ok := applied(t, name+", Undo", edited.fsys, edited.plan.Undo)
	if !ok {
		return
	}
	same(t, name+", Undo", fsys, undone.fsys)
	if redone, ok := applied(t, name+", Undo of the Undo", undone.fsys, undone.plan.Undo); ok {
		same(t, name+", Undo of the Undo", edited.fsys, redone.fsys)
	}
}

type appliedPlan struct {
	plan *edit.Plan
	fsys mapFS
}

// applied is ops applied to fsys: the plan and the files it writes; each write is minimal (M6).
func applied(t *testing.T, name string, fsys mapFS, ops []edit.Operation) (appliedPlan, bool) {
	t.Helper()
	s := open(t, fsys, nil, "", "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
	if err != nil {
		t.Errorf("%s: %+v: %v", name, ops, err)
		return appliedPlan{}, false
	}
	for _, ch := range plan.Changes {
		checkWritten(t, plan, ch)
	}
	return appliedPlan{plan: plan, fsys: written(fsys, plan)}, true
}

// sameValues reports every value of want that got does not hold (API.md E22: values, not text).
func sameValues(t *testing.T, name string, want, got mapFS) {
	t.Helper()
	w, g := values(open(t, want, nil, "", "d"), false), values(open(t, got, nil, "", "d"), false)
	for _, k := range slices.Sorted(maps.Keys(w)) {
		if g[k] != w[k] {
			t.Errorf("%s: %s = %s, want %s", name, k, g[k], w[k])
		}
	}
}

// sameFiles reports every file of want that got does not hold byte for byte.
func sameFiles(t *testing.T, name string, want, got mapFS) {
	t.Helper()
	for _, f := range slices.Sorted(maps.Keys(want)) {
		var data []byte
		if g, ok := got[f]; ok {
			data = g.Data
		}
		if string(data) != string(want[f].Data) {
			t.Errorf("%s: %s does not come back byte for byte:\n%s", name, f, data)
		}
	}
}
