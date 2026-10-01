package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/wire"
)

// chainSrc types a field by a field of a record field (TYPES.md 11.1), and that record's own
// field by its sibling.
const chainSrc = `package d

/// What a quest counts.
enum Goal { kill, collect }

/// What a quest aims at, by goal.
type Target(g: Goal) = match g {
  kill => String(1..)
  collect => Int(1..)
}

/// A part.
record Part {
  /// Its kind.
  kind: Goal
  /// Its aim.
  aim: Target(kind)?
}

/// A quest.
record Quest {
  /// What it counts.
  goal: Goal
  /// Its part.
  sub: Part
  /// Extra, by the part's kind.
  extra: Target(sub.kind)?
}

/// The quests.
let quests: table Quest = {
  slay { goal: kill, sub: { kind: kill, aim: "a" }, extra: "x" }
  other { goal: kill, sub: { kind: kill } }
}
`

// refSrc types a field by a field of the entry a ref names.
const refSrc = `package d

/// Kinds.
enum Kind { beast, thing }

/// A monster.
record Monster {
  /// Its kind.
  kind: Kind
}

/// The monsters.
let monsters: table Monster = {
  wolf { kind: beast }
  rock { kind: thing }
}

/// An aim, by monster.
type Aim(m: Monster) = match m.kind {
  beast => String(1..)
  thing => Int(1..)
}

/// A quest.
record Quest {
  /// Its foe.
  foe: ref monsters
  /// Its aim.
  aim: Aim(foe)?
}

/// The quests.
let quests: table Quest = {
  slay { foe: wolf, aim: "x" }
}
`

// prSrc types a record's fields by its own field and by its parameter, which an enclosing
// record's field gives.
const prSrc = `package d

/// What a quest counts.
enum Goal { kill, collect }

/// What a quest aims at, by goal.
type Target(g: Goal) = match g {
  kill => String(1..)
  collect => Int(1..)
}

/// A part.
record Sub(g: Goal) {
  /// Its kind.
  kind: Goal
  /// Its aim.
  aim: Target(kind)?
  /// Its other aim, by the quest's goal.
  far: Target(g)?
}

/// A quest.
record Quest {
  /// What it counts.
  goal: Goal
  /// Its part.
  sub: Sub(goal)
}

/// The quests.
let quests: table Quest = {
  slay { goal: kill, sub: { kind: kill } }
}
`

// twoDrvSrc types a field by two fields, through nested type functions.
const twoDrvSrc = `package d

/// What a quest counts.
enum Goal { kill, collect }

/// By flag, for kill.
type KF(f: Bool) = match f {
  false => String(1..)
  true => Int(1..9)
}

/// By flag, for collect.
type CF(f: Bool) = match f {
  false => Int(10..)
  true => Bool
}

/// By goal and flag.
type T2(g: Goal, f: Bool) = match g {
  kill => KF(f)
  collect => CF(f)
}

/// A quest.
record Quest {
  /// Goal.
  goal: Goal
  /// Flag.
  flag: Bool = false
  /// Aim.
  aim: T2(goal, flag)?
}

/// The quests.
let quests: table Quest = {
  slay { goal: kill, aim: "wolf" }
  other { goal: collect, aim: 12 }
}
`

func srcFS(src string) func() mapFS {
	return func() mapFS { return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(src)} }
}

type undoCase struct {
	name string
	fsys func() mapFS
	ops  []edit.Operation
}

// API.md E1, E22 over E23's order (log-2026-09-29 U-E22-r), E15, E16, M6: drivers and their values
// changed in any order, structural operations between, through nested functions, a ref, a record
// parameter or field, apply as E1 types them, and Undo and its Undo round-trip.
func TestUndoDependentRequests(t *testing.T) {
	collect, kill := edit.Member("collect"), edit.Member("kill")
	remove := func(path string) edit.Operation { return edit.Operation{Kind: edit.OpRemove, Path: path} }
	two := srcFS(twoDrvSrc)
	for _, c := range []undoCase{
		{"record field driver", srcFS(chainSrc), []edit.Operation{
			setAt("quests.slay.goal", collect), setAt("quests.slay.extra", edit.Str("y")), setAt("quests.slay.sub.kind", collect),
		}},
		{"record field driver, its sibling", srcFS(chainSrc), []edit.Operation{setAt("quests.slay.sub.kind", collect), setAt("quests.slay.extra", edit.Int(4))}},
		{"remove between", questFS, []edit.Operation{setAt("quests.slay.goal", collect), remove("quests.gather"), setAt("quests.slay.target", edit.Int(25))}},
		{"remove after", questFS, []edit.Operation{setAt("quests.slay.target", edit.Str("boar")), setAt("quests.slay.goal", collect), remove("quests.gather")}},
		{"rename after", questFS, []edit.Operation{
			setAt("quests.slay.target", edit.Str("boar")), setAt("quests.slay.goal", collect),
			{Kind: edit.OpRename, Path: "quests.gather", Key: edit.Key("pick")},
		}},
		{"list remove after the driver", questFS, []edit.Operation{setAt("quests.slay.goal", collect), remove("quests.slay.rewards[0]")}},
		{"ref driver", srcFS(refSrc), []edit.Operation{setAt("monsters.wolf.kind", edit.Member("thing")), setAt("quests.slay.aim", edit.Int(5))}},
		{"ref foe", srcFS(refSrc), []edit.Operation{setAt("quests.slay.foe", edit.Source("rock")), setAt("quests.slay.aim", edit.Int(5))}},
		{"parameter, own driver after", srcFS(prSrc), []edit.Operation{
			setAt("quests.slay.goal", collect), setAt("quests.slay.sub.aim", edit.Str("y")), setAt("quests.slay.sub.kind", collect),
		}},
		{"parameter cascade", srcFS(prSrc), []edit.Operation{setAt("quests.slay.sub.far", edit.Str("y")), setAt("quests.slay.goal", collect)}},
		{"parameter driver then field", srcFS(prSrc), []edit.Operation{setAt("quests.slay.goal", collect), setAt("quests.slay.sub.far", edit.Int(5))}},
		{"dep, driver, dep", questFS, []edit.Operation{
			setAt("quests.slay.target", edit.Str("boar")), setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25)),
		}},
		{"driver back", questFS, []edit.Operation{setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25)), setAt("quests.slay.goal", kill)}},
		{"driver back, no dependent", questFS, []edit.Operation{setAt("quests.slay.goal", collect), setAt("quests.slay.goal", kill)}},
		{"driver back, dependent reset", questFS, []edit.Operation{
			setAt("quests.slay.goal", collect), {Kind: edit.OpReset, Path: "quests.slay.target"}, setAt("quests.slay.goal", kill),
		}},
		{"driver there and back, dependent after", questFS, []edit.Operation{
			setAt("quests.slay.goal", collect), setAt("quests.slay.goal", kill), setAt("quests.slay.target", edit.Str("boar")),
		}},
		{"list remove then set", questFS, []edit.Operation{
			setAt("quests.slay.items", edit.Bool(true)), remove("quests.slay.rewards[0]"), setAt("quests.slay.rewards[0]", edit.Str("gem")),
		}},
		{"list insert", questFS, []edit.Operation{
			setAt("quests.slay.items", edit.Bool(true)), setAt("quests.slay.rewards", edit.Source(`[]`)),
			{Kind: edit.OpInsert, Path: "quests.slay.rewards", Index: 0, Value: edit.Str("gem")},
		}},
		{"list add", questFS, []edit.Operation{
			setAt("quests.slay.items", edit.Bool(true)), setAt("quests.slay.rewards", edit.Source(`["a"]`)),
			{Kind: edit.OpAdd, Path: "quests.slay.rewards", Value: edit.Str("gem")},
		}},
		{"entry added, then its driver and dependent", questFS, []edit.Operation{
			{Kind: edit.OpAddEntry, Path: "quests", Key: edit.Key("hunt"), Value: edit.Source(`{ goal: kill, target: "x", act: wait { turns: 1 } }`)},
			setAt("quests.hunt.goal", collect), setAt("quests.hunt.target", edit.Int(4)),
		}},
		{"entry removed and added back", questFS, []edit.Operation{
			setAt("quests.slay.goal", collect), remove("quests.slay"),
			{Kind: edit.OpAddEntry, Path: "quests", Key: edit.Key("slay"), Value: edit.Source(`{ goal: kill, target: "y", act: wait { turns: 1 } }`)},
		}},
		{"SetCase, then its driver", questFS, []edit.Operation{
			{Kind: edit.OpSetCase, Path: "quests.slay.act", Case: "hunt", Value: edit.Obj{"goal": collect}},
			setAt("quests.slay.act.target", edit.Int(5)), setAt("quests.slay.act.goal", kill),
		}},
		{"case driver, then the case whole", questFS, []edit.Operation{
			setAt("quests.gather.act.goal", collect), setAt("quests.gather.act.target", edit.Int(5)),
			setAt("quests.gather.act", edit.Source(`hunt { goal: kill, target: "deer" }`)),
		}},
		{"two drivers, both first", two, []edit.Operation{
			setAt("quests.slay.goal", collect), setAt("quests.slay.flag", edit.Bool(true)), setAt("quests.slay.aim", edit.Bool(true)),
		}},
		{"two drivers, interleaved", two, []edit.Operation{
			setAt("quests.slay.flag", edit.Bool(true)), setAt("quests.slay.aim", edit.Int(3)),
			setAt("quests.slay.goal", collect), setAt("quests.slay.aim", edit.Bool(false)),
		}},
		{"two drivers, cascade", two, []edit.Operation{
			setAt("quests.slay.goal", collect), setAt("quests.slay.aim", edit.Int(50)), setAt("quests.slay.flag", edit.Bool(true)),
		}},
		{"two drivers, one there and back", two, []edit.Operation{
			setAt("quests.slay.flag", edit.Bool(true)), setAt("quests.slay.aim", edit.Int(3)), setAt("quests.slay.flag", edit.Bool(false)),
			setAt("quests.slay.goal", collect), setAt("quests.slay.aim", edit.Int(11)),
		}},
	} {
		undoTwice(t, c.name, c.fsys(), c.ops)
	}
}

// API.md E1, E22 (log-2026-09-29 U-E22-r), E15, M6, M8: the same in JSON sources, held values
// included (DECISIONS 175): each file comes back byte for byte, and the Undo of the Undo gives
// the edited files back.
func TestUndoDependentJSON(t *testing.T) {
	m := func(s string) edit.Lit { return edit.Member(s) }
	for _, c := range []undoCase{
		{"driver then dependent", depFS, []edit.Operation{setAt("root.tag.goal", m("kill")), setAt("root.tag.pick", edit.Key("wolf"))}},
		{"dependent then driver", depFS, []edit.Operation{setAt("root.tag.pick", m("green")), setAt("root.tag.goal", m("kill"))}},
		{"driver, dependent, driver", depFS, []edit.Operation{
			setAt("root.tag.goal", m("kill")), setAt("root.tag.pick", edit.Key("wolf")), setAt("root.tag.goal", m("paint")),
		}},
		{"driver, dependent, driver there and back", depFS, []edit.Operation{
			setAt("root.tag.goal", m("kill")), setAt("root.tag.pick", edit.Key("wolf")), setAt("root.tag.goal", m("count")), setAt("root.tag.goal", m("kill")),
		}},
		{"held: dependent then driver", undoFS, []edit.Operation{setAt("root.tag.pick", edit.FromJSON(`"q"`)), setAt("root.tag.goal", m("paint"))}},
		{"held: dependent then ref driver", undoFS, []edit.Operation{
			setAt("objectives.o2.target", edit.FromJSON(`"elsewhere"`)), setAt("objectives.o2.def", edit.Key("art")),
		}},
		{"dependent map entry then driver", depFS, []edit.Operation{
			{Kind: edit.OpAddEntry, Path: "root.tag.byPick", Key: m("green"), Value: edit.Int(1)}, setAt("root.tag.goal", m("kill")),
		}},
		{"driver then dependent map entry", depFS, []edit.Operation{
			setAt("root.tag.goal", m("kill")), {Kind: edit.OpAddEntry, Path: "root.tag.byPick", Key: edit.Key("wolf"), Value: edit.Int(1)},
		}},
		{"list: add, driver, remove other, dependent", depFS, []edit.Operation{
			{Kind: edit.OpAdd, Path: "root.objs", Value: edit.Source("{ def: hunt, target: wolf }")},
			setAt("root.objs[0].def", edit.Key("hunt")), {Kind: edit.OpRemove, Path: "root.objs[1]"}, setAt("root.objs[0].target", edit.Key("bear")),
		}},
		{"list: insert first, driver, dependent", depFS, []edit.Operation{
			{Kind: edit.OpInsert, Path: "root.objs", Index: 0, Value: edit.Source("{ def: hunt, target: wolf }")},
			setAt("root.objs[1].def", edit.Key("hunt")), setAt("root.objs[1].target", edit.Key("bear")),
		}},
	} {
		undoTwiceFiles(t, c.name, c.fsys(), c.ops)
	}
}

// API.md E1 (DECISIONS 237): an operation whose path lies under a value an earlier operation of
// the request left uncomputed is ErrNoValue, even when the final state would be valid: a JSON
// record whose driver changed no longer decodes its dependent value, and its root with it.
func TestE1UncomputedByEarlierOperation(t *testing.T) {
	m := func(s string) edit.Lit { return edit.Member(s) }
	for _, c := range []undoCase{
		{"ref driver then dependent", depFS, []edit.Operation{setAt("objectives.o1.def", edit.Key("art")), setAt("objectives.o1.target", m("green"))}},
		{"ref driver then a field beside", depFS, []edit.Operation{setAt("objectives.o1.def", edit.Key("art")), setAt("objectives.o1.note", edit.Str("x"))}},
		{"ref driver then an element", depFS, []edit.Operation{
			setAt("objectives.o1.def", edit.Key("art")), {Kind: edit.OpAdd, Path: "objectives.o1.alts", Value: m("red")},
		}},
		{"held: driver then dependent", undoFS, []edit.Operation{setAt("root.tag.goal", m("paint")), setAt("root.tag.pick", m("green"))}},
		{"held: driver twice", undoFS, []edit.Operation{setAt("root.tag.goal", m("paint")), setAt("root.tag.goal", m("count"))}},
		{"held: ref driver then dependent", undoFS, []edit.Operation{setAt("objectives.o2.def", edit.Key("art")), setAt("objectives.o2.target", m("red"))}},
	} {
		s := open(t, c.fsys(), nil, "", "d")
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		var oe *edit.OpError
		if !errors.As(err, &oe) || oe.Index != len(c.ops)-1 || !errors.Is(err, edit.ErrNoValue) {
			t.Errorf("%s: %v, want ErrNoValue at operation %d", c.name, err, len(c.ops)-1)
		}
	}
}

const layerQuestSrc = `package d

/// What a quest counts.
enum Goal { kill, collect }

/// What a quest aims at, by goal.
type Target(g: Goal) = match g {
  kill => String(1..)
  collect => Int(1..)
}

/// A quest.
record Quest {
  /// What it counts.
  goal: Goal
  /// What it aims at.
  target: Target(goal)?
  /// A note.
  note: String = ""
}

/// One quest.
let q: Quest = { goal: kill, target: "wolf" }

/// Another quest.
let r: Quest = { goal: kill, target: "wolf" }
`

const layerQuestDev = `package d
layer dev

amend r {
  goal: collect
  target: 5
}
`

// API.md E22, W11 (log-2026-09-29 U-E22-r): under an edit layer, which amends a root field by
// field, the Undo restores the region a whole Set may not write field by field, a field before
// the ones it types; the values with the layer active come back, then the edited ones.
func TestUndoDependentLayer(t *testing.T) {
	fs := func() mapFS {
		return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(layerQuestSrc), "law/d/dev.layer.canon": file(layerQuestDev)}
	}
	collect, kill := edit.Member("collect"), edit.Member("kill")
	for _, c := range []undoCase{
		{"driver then dependent", fs, []edit.Operation{setAt("q.goal", collect), setAt("q.target", edit.Int(25))}},
		{"dependent then driver", fs, []edit.Operation{setAt("q.target", edit.Str("boar")), setAt("q.goal", collect)}},
		{"amended driver then dependent", fs, []edit.Operation{setAt("r.goal", kill), setAt("r.target", edit.Str("x"))}},
		{"amendment reset, dependent", fs, []edit.Operation{{Kind: edit.OpReset, Path: "r.goal"}, setAt("r.target", edit.Str("x"))}},
		{"both amendments reset", fs, []edit.Operation{{Kind: edit.OpReset, Path: "r.goal"}, {Kind: edit.OpReset, Path: "r.target"}}},
	} {
		layerUndoTwice(t, c.name, c.fsys(), c.ops)
	}
}

// layerUndoTwice is undoTwice with the layer dev active and edited, values compared.
func layerUndoTwice(t *testing.T, name string, fsys mapFS, ops []edit.Operation) {
	t.Helper()
	layers := []string{"dev"}
	apply := func(n string, fs mapFS, ops []edit.Operation) (*edit.Plan, mapFS, bool) {
		s := open(t, fs, layers, "dev", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
		if err != nil {
			t.Errorf("%s: %+v: %v", n, ops, err)
			return nil, nil, false
		}
		for _, ch := range plan.Changes {
			checkWritten(t, plan, ch)
		}
		return plan, written(fs, plan), true
	}
	same := func(n string, want, got mapFS) {
		w, g := values(open(t, want, layers, "dev", "d"), false), values(open(t, got, layers, "dev", "d"), false)
		for k, v := range w { //canon:unordered each value is compared alone
			if g[k] != v {
				t.Errorf("%s: %s = %s, want %s", n, k, g[k], v)
			}
		}
	}
	p1, f1, ok := apply(name, fsys, ops)
	if !ok {
		return
	}
	p2, f2, ok := apply(name+", Undo", f1, p1.Undo)
	if !ok {
		return
	}
	same(name+", Undo", fsys, f2)
	if _, f3, ok := apply(name+", Undo of the Undo", f2, p2.Undo); ok {
		same(name+", Undo of the Undo", f1, f3)
	}
}

// log-2026-09-29 U-E22-r: where E23's order would fail, the Undo restores the smallest item
// enclosing the value and the fields its type is computed from, whole, and nothing else of it.
func TestUndoRegionIsSmallest(t *testing.T) {
	for _, c := range []struct {
		undoCase
		want string
	}{
		{undoCase{"a quest", questFS, []edit.Operation{setAt("quests.slay.goal", edit.Member("collect")), setAt("quests.slay.target", edit.Int(25))}},
			"d:quests.slay"},
		{undoCase{"a case", questFS, []edit.Operation{setAt("quests.gather.act.goal", edit.Member("collect")), setAt("quests.gather.act.target", edit.Int(5))}},
			"d:quests.gather.act"},
	} {
		edited, ok := applied(t, c.name, c.fsys(), c.ops)
		if ok && (len(edited.plan.Undo) != 1 || edited.plan.Undo[0].Kind != edit.OpSet || edited.plan.Undo[0].Path != c.want) {
			t.Errorf("%s: Undo %+v, want one Set of %s", c.name, edited.plan.Undo, c.want)
		}
	}
}

// NFR-01, log-2026-09-29 U-E22-r: only a request of several operations touching a dependent
// field or a driver runs its Undo in memory, which analyzes the edit's result a second time;
// a single operation, cascade or not, and several that touch none never do.
func TestUndoVerifiedOnlyWhenNeeded(t *testing.T) {
	collect := edit.Member("collect")
	for _, c := range []struct {
		name string
		ops  []edit.Operation
		dry  bool
	}{
		{"one Set of a driver, with its cascade", []edit.Operation{setAt("quests.slay.goal", collect)}, false},
		{"one Set of a dependent", []edit.Operation{setAt("quests.slay.target", edit.Str("boar"))}, false},
		{"several Sets of plain fields", []edit.Operation{setAt("quests.slay.note", edit.Str("a")), setAt("quests.gather.note", edit.Str("b"))}, false},
		{"a driver and its dependent", []edit.Operation{setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25))}, true},
	} {
		s := open(t, questFS(), nil, "", "d")
		seen := map[*build.Analysis]int{}
		s.env.Host = func(a *build.Analysis) wire.Host {
			seen[a]++
			return hostOf(a)
		}
		if _, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops}); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		dry := false
		for _, n := range seen { //canon:unordered an any-of test
			dry = dry || n > 1
		}
		if dry != c.dry {
			t.Errorf("%s: Undo run in memory %v, want %v", c.name, dry, c.dry)
		}
	}
}
