package edit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/edit"
)

// scopeSrc is a package whose fields have wire rules of their own: units, int and a none marker,
// in a JSON source and in Canon.
const scopeSrc = `/// D.
package d

/// A buff.
record Buff {
  /// How long it lasts.
  wait: Duration @json("waitSec", unit: s)
  /// When it ticks.
  ticks: [Duration] = [] @json(unit: m)
  /// Whether it stacks.
  stacks: Bool = false @json(int)
  /// Its parent, if any.
  parent: String? = "base" @json(none: "")
  /// Waits by name.
  byName: {String: Duration} = {} @json(unit: s)
}

/// Loaded.
let buff: Buff = load("buff.json")

/// In Canon.
let canonBuff: Buff = { wait: 30s }
`

const scopeJSON = `{
  "waitSec": 30,
  "ticks": [
    1
  ],
  "parent": "p"
}
`

func scopeFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(scopeSrc), "law/d/buff.json": file(scopeJSON)}
}

// API.md E24, V1, E22 (log-2026-09-29 M4 B10): FromJSON is read by the destination field's own
// wire rules, as the file writes it: a unit, an element's unit, int, the none marker; the Undo
// gives each file back byte for byte.
func TestFromJSONFieldScope(t *testing.T) {
	for _, c := range []struct {
		name, file string
		op         edit.Operation
		want       []string
	}{
		{"JSON unit", "d/buff.json", edit.Operation{Kind: edit.OpSet, Path: "buff.wait", Value: edit.FromJSON(`2`)}, []string{`"waitSec": 2`}},
		{"JSON element unit", "d/buff.json", edit.Operation{Kind: edit.OpAdd, Path: "buff.ticks", Value: edit.FromJSON(`3`)}, []string{"    1,\n    3\n"}},
		{"Canon unit", "d/d.canon", edit.Operation{Kind: edit.OpSet, Path: "canonBuff.wait", Value: edit.FromJSON(`2`)}, []string{"wait: 2s"}},
		{"Canon list unit", "d/d.canon", edit.Operation{Kind: edit.OpSet, Path: "canonBuff.ticks", Value: edit.FromJSON(`[1, 2]`)}, []string{"ticks: [1m, 2m]"}},
		{"Canon int", "d/d.canon", edit.Operation{Kind: edit.OpSet, Path: "canonBuff.stacks", Value: edit.FromJSON(`1`)}, []string{"stacks: true"}},
		{"Canon none marker", "d/d.canon", edit.Operation{Kind: edit.OpSet, Path: "canonBuff.parent", Value: edit.FromJSON(`""`)}, []string{"parent: none"}},
	} {
		applyDepOn(t, scopeFS(), symCase{c.name, c.op, c.file, c.want})
		roundTrip(t, c.name, scopeFS(), []edit.Operation{c.op})
	}
}

// API.md V1 (log-2026-09-29 M4 B10-r2): a Duration a JSON source cannot write as a whole number of
// its field's unit is a *ValueError, a map's value too, whole or added; into a .canon source it is
// valid Canon, left to the re-check's E8102 (E18).
func TestUnitFractionRefused(t *testing.T) {
	s := open(t, scopeFS(), nil, "", "d")
	for _, c := range []struct {
		op      edit.Operation
		refused bool
	}{
		{edit.Operation{Kind: edit.OpAddEntry, Path: "buff.byName", Key: edit.Key("a"), Value: edit.Dur(1500 * time.Millisecond)}, true},
		{edit.Operation{Kind: edit.OpSet, Path: "buff.byName", Value: edit.Source("{ a: 1500ms }")}, true},
		{edit.Operation{Kind: edit.OpAddEntry, Path: "buff.byName", Key: edit.Key("a"), Value: edit.Dur(2 * time.Second)}, false},
		{edit.Operation{Kind: edit.OpSet, Path: "canonBuff.wait", Value: edit.Dur(1500 * time.Millisecond)}, false},
	} {
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		var ve *edit.ValueError
		if errors.As(err, &ve) != c.refused || !c.refused && err != nil {
			t.Errorf("%s %v: %v, want refused %v", c.op.Path, c.op.Value, err, c.refused)
		}
	}
}

// keyedUnitSrc is a map keyed by a dependent type whose Durations are in seconds.
const keyedUnitSrc = `/// D.
package d

/// A goal.
enum Goal { kill, paint }

/// A colour.
enum Color { red, green }

/// A mob.
record Mob {
  /// Hit points.
  hp: Int = 1
}

/// The mobs.
let mobs: table Mob = { wolf {}, bear {} }

/// What a goal picks.
type Pick(g: Goal) = match g {
  kill => ref mobs
  paint => Color
}

/// A goal definition.
record Def {
  /// Its goal.
  goal: Goal
}

/// The definitions.
let defs: table Def = { hunt { goal: kill }, art { goal: paint } }

/// What a definition holds.
type Hold(d: Def) = match d.goal {
  kill => ref mobs
  paint => Duration
}

/// A wait per pick.
record Waits {
  /// Its goal.
  goal: Goal = kill
  /// How long each pick waits.
  byPick: {Pick(goal): Duration} = {} @json(unit: s)
  /// What each definition holds.
  holds: {d in defs: Hold(d)} = {}
}

/// Loaded.
let waits: Waits = load("waits.json")
`

// API.md E15, E22, E23 (log-2026-09-29 M4 B10): a keyed map in seconds that E15 drops comes back
// by the Undo with the same values: what the Undo carries reads back in the unit it was written in.
func TestUndoKeyedUnits(t *testing.T) {
	roundTrip(t, "keyed map in seconds", keyedUnitFS(), []edit.Operation{{Kind: edit.OpSet, Path: "waits.goal", Value: edit.Member("paint")}})
}

func keyedUnitFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(keyedUnitSrc),
		"law/d/waits.json": file("{\n  \"byPick\": {\n    \"wolf\": 30\n  }\n}\n")}
}

// API.md E24, E22; WIRE.md 4.1 (log-2026-09-29 M4 B10-r): FromJSON into a map keyed by a dependent
// type is read in its field's unit, 30 as 30 s, whole or entry by entry; into a dependent map's
// value, in the branch its key selects; each Undo gives the file back byte for byte.
func TestFromJSONDependentScope(t *testing.T) {
	for _, c := range []struct {
		name string
		op   edit.Operation
		want string
	}{
		{"keyed map", edit.Operation{Kind: edit.OpSet, Path: "waits.byPick", Value: edit.FromJSON(`{"wolf": 30, "bear": 2}`)}, `"bear": 2`},
		{"keyed map entry", edit.Operation{Kind: edit.OpAddEntry, Path: "waits.byPick", Key: edit.Key("bear"), Value: edit.FromJSON(`30`)}, `"bear": 30`},
		{"dependent map value", edit.Operation{Kind: edit.OpAddEntry, Path: "waits.holds", Key: edit.Key("art"), Value: edit.FromJSON(`2500`)}, `"art": 2500`},
		{"dependent map ref", edit.Operation{Kind: edit.OpAddEntry, Path: "waits.holds", Key: edit.Key("hunt"), Value: edit.FromJSON(`"wolf"`)}, `"hunt": "wolf"`},
	} {
		applyDepOn(t, keyedUnitFS(), symCase{c.name, c.op, "d/waits.json", []string{c.want}})
		roundTrip(t, c.name, keyedUnitFS(), []edit.Operation{c.op})
	}
}
