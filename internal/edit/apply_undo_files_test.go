package edit_test

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

const filesSrc = `package d

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

/// The quests, each in its file.
@files("q/{goal}/{id}.canon")
let quests: table Quest = {}
`

func filesFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(filesSrc),
		"law/d/q/kill/slay.canon":      file("package d\n\n/// Slay.\nentry quests.slay { goal: kill, target: \"wolf\" }\n"),
		"law/d/q/collect/gather.canon": file("package d\n\n/// Gather.\nentry quests.gather { goal: collect, target: 10 }\n"),
	}
}

// API.md E11, E22, N1, N8 (log-2026-10-01 M4.1 review rulings): in a table whose entries live in
// files, an entry's file is part of what an Undo gives back: a renamed entry is restored, then
// renamed back with its file; every entry file comes back at its path.
func TestUndoEntryFiles(t *testing.T) {
	collect, kill := edit.Member("collect"), edit.Member("kill")
	ren := func(p, k string) edit.Operation {
		return edit.Operation{Kind: edit.OpRename, Path: p, Key: edit.Key(k)}
	}
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"driver then dependent", []edit.Operation{setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25))}},
		{"dependent then driver", []edit.Operation{setAt("quests.slay.target", edit.Str("boar")), setAt("quests.slay.goal", collect)}},
		{"removed and added again", []edit.Operation{{Kind: edit.OpRemove, Path: "quests.slay"},
			{Kind: edit.OpAddEntry, Path: "quests", Key: edit.Key("slay"), Value: edit.Source(`{ goal: collect, target: 3 }`)},
			setAt("quests.slay.goal", kill), setAt("quests.slay.target", edit.Str("y"))}},
		{"renamed, then edited", []edit.Operation{{Kind: edit.OpRename, Path: "quests.slay", Key: edit.Key("slain")},
			setAt("quests.slain.goal", collect), setAt("quests.slain.target", edit.Int(2))}},
		{"swapped, then edited", []edit.Operation{ren("quests.slay", "tmp"), ren("quests.gather", "slay"), ren("quests.tmp", "gather"),
			setAt("quests.slay.goal", kill), setAt("quests.slay.target", edit.Str("x")), setAt("quests.gather.goal", collect), setAt("quests.gather.target", edit.Int(3))}},
		{"swapped, edits between", []edit.Operation{setAt("quests.slay.goal", collect), ren("quests.slay", "tmp"), ren("quests.gather", "slay"),
			setAt("quests.tmp.target", edit.Int(7)), ren("quests.tmp", "gather"), setAt("quests.slay.goal", kill), setAt("quests.slay.target", edit.Str("y"))}},
		{"renamed along a chain", []edit.Operation{ren("quests.slay", "a"), setAt("quests.a.goal", collect), ren("quests.a", "b"),
			setAt("quests.b.target", edit.Int(1)), ren("quests.b", "c")}},
	} {
		fs := filesFS()
		plan, f1, ok := applyIn(t, c.name, fs, undoView{}, c.ops)
		if !ok {
			continue
		}
		_, f2, ok := applyIn(t, c.name+", Undo", f1, undoView{}, plan.Undo)
		if !ok {
			continue
		}
		sameIn(t, c.name+", Undo", fs, f2, nil)
		if w, g := slices.Sorted(maps.Keys(fs)), slices.Sorted(maps.Keys(f2)); !slices.Equal(w, g) {
			t.Errorf("%s, Undo: files %v, want %v", c.name, g, w)
		}
	}
}

// refRegionSrc types a quest's aim by its foe's kind, and a monster's size by its own kind.
const refRegionSrc = `package d

/// Kinds.
enum Kind { beast, thing }

/// Size by kind.
type Size(k: Kind) = match k {
  beast => Int(1..)
  thing => String(1..)
}

/// A monster.
record Monster {
  /// Its kind.
  kind: Kind
  /// Its size.
  size: Size(kind)?
}

/// The monsters.
let zmon: table Monster = {
  wolf { kind: beast, size: 3 }
  rock { kind: thing, size: "big" }
}

/// An aim, by monster.
type Aim(m: Monster) = match m.kind {
  beast => String(1..)
  thing => Int(1..)
}

/// A quest.
record Quest {
  /// Its foe.
  foe: ref zmon
  /// Its aim.
  aim: Aim(foe)?
}

/// The quests.
let aq: table Quest = {
  slay { foe: wolf, aim: "x" }
}
`

// API.md E22 (log-2026-09-29 U-E22-r): two regions, one typed through a ref into the other, are
// both restored whole and every value comes back.
func TestUndoRegionsThroughRef(t *testing.T) {
	fs := srcFS(refRegionSrc)
	undoIn(t, undoSpec{name: "two regions", fsys: fs(), views: [][]string{nil}, ops: []edit.Operation{
		setAt("zmon.wolf.kind", edit.Member("thing")), setAt("zmon.wolf.size", edit.Str("huge")), setAt("aq.slay.aim", edit.Int(5)),
	}})
	undoIn(t, undoSpec{name: "one region", fsys: fs(), views: [][]string{nil}, ops: []edit.Operation{
		setAt("zmon.wolf.kind", edit.Member("thing")), setAt("aq.slay.aim", edit.Int(5)),
	}})
}

// API.md E1, E22, M8, DECISIONS 175: more JSON requests, held values beside and inside, a whole
// record then its field; the files come back byte for byte.
func TestUndoJSONMore(t *testing.T) {
	m := func(s string) edit.Lit { return edit.Member(s) }
	for _, c := range []undoCase{
		{"held record beside", undoFS, []edit.Operation{setAt("root.objs[0].def", edit.Key("hunt")), setAt("root.objs[0].target", edit.Key("bear"))}},
		{"whole record, then its field", undoFS, []edit.Operation{
			setAt("root.tag", edit.FromJSON(`{"goal":"kill","pick":"wolf"}`)), setAt("root.tag.pick", edit.Key("bear")),
		}},
		{"three records", undoFS, []edit.Operation{
			setAt("root.aims[hunt]", edit.Key("bear")), setAt("root.objs[1].def", edit.Key("tour")), setAt("root.tag.goal", m("visit")),
		}},
	} {
		undoTwiceFiles(t, c.name, c.fsys(), c.ops)
	}
}

// API.md E22, W9: a field a spread supplies, then its dependent, come back, and the entry keeps
// following its template.
func TestUndoUnderSpread(t *testing.T) {
	src := strings.Replace(questSrc, `let quests: table Quest = {
  slay { goal: kill, target: "wolf", bonus: "bear", act: wait { turns: 2 }, rewards: [100, 50] }`, `/// A template.
let tmpl: Quest = { goal: kill, target: "wolf", bonus: "bear", note: "tmpl", act: wait { turns: 2 }, rewards: [100, 50] }

/// The quests.
let quests: table Quest = {
  slay { ...tmpl, target: "deer" }`, 1)
	fs := srcFS(src)()
	plan, f1, ok := applyIn(t, "spread", fs, undoView{}, []edit.Operation{setAt("quests.slay.goal", edit.Member("collect")), setAt("quests.slay.target", edit.Int(25))})
	if !ok {
		return
	}
	_, f2, ok := applyIn(t, "spread, Undo", f1, undoView{}, plan.Undo)
	if !ok || !sameIn(t, "spread, Undo", fs, f2, nil) {
		return
	}
	follow := []edit.Operation{setAt("tmpl.note", edit.Str("changed"))}
	_, g1, ok1 := applyIn(t, "template, before", fs, undoView{}, follow)
	_, g2, ok2 := applyIn(t, "template, after the Undo", f2, undoView{}, follow)
	if ok1 && ok2 {
		sameIn(t, "the entry follows its template", g1, g2, nil)
	}
}

// API.md E4, E22, E23: a Retire, which has no inverse, beside a driver and its dependent: the
// Undo gives back every other value.
func TestUndoBesideRetire(t *testing.T) {
	src := strings.Replace(questSrc, "let quests: table Quest", "let quests: stable table Quest", 1)
	collect := edit.Member("collect")
	retire := edit.Operation{Kind: edit.OpRetire, Path: "quests.slay"}
	for _, ops := range [][]edit.Operation{
		{retire, setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25))},
		{setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25)), retire},
	} {
		undoIn(t, undoSpec{name: "retire", fsys: srcFS(src)(), ops: ops, views: [][]string{nil}})
	}
}

// API.md E22, N1, N2, N3, N4 (log-2026-10-01 M4.1 rulings, a): a hand-placed entry removed, or
// removed and added back, in a request that changes a driver and its dependent verifies; its Undo
// recreates it at its template's path.
func TestUndoRecreatesByTemplate(t *testing.T) {
	kill := edit.Member("kill")
	rm := edit.Operation{Kind: edit.OpRemove, Path: "quests.slay"}
	readd := edit.Operation{Kind: edit.OpAddEntry, Path: "quests", Key: edit.Key("slay"), Value: edit.Source(`{ goal: collect, target: 3 }`)}
	for _, ops := range [][]edit.Operation{
		{rm, setAt("quests.gather.goal", kill), setAt("quests.gather.target", edit.Str("x"))},
		{rm, readd, setAt("quests.slay.goal", kill), setAt("quests.slay.target", edit.Str("z"))},
	} {
		fs := filesFS()
		fs["law/d/q/hand/slay.canon"] = fs["law/d/q/kill/slay.canon"]
		delete(fs, "law/d/q/kill/slay.canon")
		plan, f1, ok := applyIn(t, "hand-placed", fs, undoView{}, ops)
		if !ok {
			continue
		}
		_, f2, ok := applyIn(t, "hand-placed, Undo", f1, undoView{}, plan.Undo)
		if !ok {
			continue
		}
		sameIn(t, "hand-placed, Undo", fs, f2, nil)
		if _, at := f2["law/d/q/kill/slay.canon"]; !at {
			t.Errorf("the Undo recreates slay at %v, want law/d/q/kill/slay.canon", slices.Sorted(maps.Keys(f2)))
		}
	}
}

// heldErrSrc's slay holds a target its goal does not take (E3802), not held from a JSON source.
var heldErrSrc = strings.Replace(questSrc, `slay { goal: kill, target: "wolf", bonus: "bear"`, `slay { goal: collect, target: "wolf", bonus: 3`, 1)

// API.md E15, E22 (log-2026-10-01 M4.1 review rulings): a value the base holds in error is left
// as the cascade drops it, one operation or several: no internal failure, the region does not
// grow, and every other value comes back.
func TestUndoBaseHeldInError(t *testing.T) {
	kill := edit.Member("kill")
	for _, ops := range [][]edit.Operation{
		{setAt("quests.slay.goal", kill)},
		{setAt("quests.slay.note", edit.Str("n")), setAt("quests.slay.goal", kill)},
		{setAt("quests.slay.goal", kill), setAt("quests.slay.bonus", edit.Str("elk"))},
	} {
		fs := srcFS(heldErrSrc)()
		s := open(t, fs, nil, "", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
		if err != nil {
			t.Errorf("%+v: %v", ops, err)
			continue
		}
		if slices.ContainsFunc(plan.Undo, func(op edit.Operation) bool { return op.Path == "d:quests" }) {
			t.Errorf("%+v: the Undo %+v restores the whole table", ops, plan.Undo)
		}
		_, f2, ok := applyIn(t, "Undo", written(fs, plan), undoView{}, plan.Undo)
		if !ok {
			continue
		}
		want := strings.Replace(heldErrSrc, `target: "wolf", `, "", 1)
		sameIn(t, "Undo", srcFS(want)(), f2, nil)
	}
}

// API.md E4, E22, E23 (DECISIONS 277): the Undo of an AddEntry into a stable table retires the
// new key, alone or beside a driver and its dependent (the Undo then verified): the entry stays,
// retired, and every other value comes back.
func TestUndoStableAddEntry(t *testing.T) {
	src := strings.Replace(questSrc, "let quests: table Quest", "let quests: stable table Quest", 1)
	collect := edit.Member("collect")
	add := edit.Operation{Kind: edit.OpAddEntry, Path: "quests", Key: edit.Key("hunt"), Value: edit.Source(`{ goal: kill, target: "elk", act: wait {} }`)}
	for _, ops := range [][]edit.Operation{
		{add},
		{add, setAt("quests.hunt.goal", collect), setAt("quests.hunt.target", edit.Int(4))},
		{setAt("quests.slay.goal", collect), add, setAt("quests.slay.target", edit.Int(25))},
	} {
		fs := srcFS(src)()
		plan, f1, ok := applyIn(t, "stable AddEntry", fs, undoView{}, ops)
		if !ok {
			continue
		}
		if slices.ContainsFunc(plan.Undo, func(op edit.Operation) bool { return op.Kind == edit.OpRemove }) ||
			!slices.ContainsFunc(plan.Undo, func(op edit.Operation) bool { return op.Kind == edit.OpRetire && op.Path == "d:quests.hunt" }) {
			t.Errorf("API.md E23, %+v: Undo %+v, want a Retire of d:quests.hunt and no Remove", ops, plan.Undo)
			continue
		}
		back, f2, ok := applyIn(t, "stable AddEntry, Undo", f1, undoView{}, plan.Undo)
		if !ok {
			continue
		}
		text := string(f2["law/d/d.canon"].Data)
		at := strings.Index(text, "\n  retired hunt {")
		if at < 0 {
			t.Errorf("API.md E23, %+v: the entry is not retired:\n%s", ops, text)
			continue
		}
		end := at + 1 + strings.IndexByte(text[at+1:], '\n')
		without := maps.Clone(f2)
		without["law/d/d.canon"] = file(text[:at] + text[end:])
		sameIn(t, "stable AddEntry, Undo, but the retired entry", fs, without, nil)
		if want := []edit.Locked{{Name: "d.quests", Key: "hunt"}}; !slices.Equal(back.Locked, want) {
			t.Errorf("API.md E20, %+v: the Undo locks %+v, want %+v", ops, back.Locked, want)
		}
	}
}

// API.md E23, E20, E22 (log-2026-10-02, G2 round 3): an entry a request adds to a stable table
// and drops with a later Set of the whole table was never locked: the Undo takes no Retire, its
// Set back leaves the entry out, and the Undo gives the file back byte for byte.
func TestUndoStableAddEntryDropped(t *testing.T) {
	src := strings.Replace(questSrc, "let quests: table Quest", "let quests: stable table Quest", 1)
	collect := edit.Member("collect")
	add := edit.Operation{Kind: edit.OpAddEntry, Path: "d:quests", Key: edit.Key("hunt"), Value: edit.Source(`{ goal: kill, target: "elk", act: wait {} }`)}
	whole := setAt("d:quests", edit.Source(`{ slay { goal: collect, target: 3, act: wait { turns: 2 } }, gather { goal: collect, target: 10, act: hunt { goal: kill, target: "boar" } } }`))
	for _, ops := range [][]edit.Operation{
		{setAt("d:quests.slay.goal", collect), add, setAt("d:quests.hunt.goal", collect), whole},
		{setAt("d:quests.slay.goal", collect), add, whole},
	} {
		fs := srcFS(src)()
		fs["law/d/canon.lock"] = file("# canon.lock v1\ntable  d.quests  gather\ntable  d.quests  slay\n")
		plan, f1, ok := applyIn(t, "stable AddEntry dropped", fs, undoView{}, ops)
		if !ok {
			continue
		}
		if slices.ContainsFunc(plan.Undo, func(op edit.Operation) bool { return op.Kind == edit.OpRetire }) {
			t.Errorf("API.md E23, %+v: Undo %+v retires an entry never locked", ops, plan.Undo)
		}
		back, f2, ok := applyIn(t, "stable AddEntry dropped, Undo", f1, undoView{}, plan.Undo)
		if !ok {
			continue
		}
		sameIn(t, "stable AddEntry dropped, Undo", fs, f2, nil)
		if got := string(f2["law/d/d.canon"].Data); got != src {
			t.Errorf("API.md E23, %+v: the file does not come back:\n%s", ops, got)
		}
		if len(back.Locked) != 0 {
			t.Errorf("API.md E20, %+v: the Undo locks %+v", ops, back.Locked)
		}
	}
}
