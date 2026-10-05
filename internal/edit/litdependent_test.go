package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// litSrc's dependent types take string and integer literals: a map keyed by a union over a type
// application, like SpecificKey, and a field whose branch is an Int or a String.
const litSrc = `/// D.
package d

/// A goal.
enum Goal { kill, paint }

/// A colour.
enum Color { red, green = "GREEN" }

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

/// A pick, or "default" for the rest.
type AnyPick(g: Goal) = Pick(g) | "default"

/// How much a goal counts.
type Amount(g: Goal) = match g {
  kill => Int
  paint => String
}

/// A tally.
record Tally {
  /// Its goal.
  goal: Goal = kill
  /// Counts by pick.
  byPick: {AnyPick(goal): Int} = {}
  /// Its amount.
  amount: Amount(goal)?
}

/// Loaded.
let tally: Tally = load("tally.json")

/// In Canon.
let canonTally: Tally = { amount: 1 }
`

const litTally = `{
  "byPick": {
    "wolf": 1
  }
}
`

func litFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(litSrc), "law/d/tally.json": file(litTally)}
}

// TYPES.md 11.4, 11.5, API.md V1, E22 (log-2026-09-29 M4 B7-r, B10): a Source's string or integer
// literal for a dependent type is kept as written when a branch takes it (a "default" key of a
// SpecificKey-like union), in JSON and Canon; each write is minimal and its Undo exact.
func TestSourceDependentLiterals(t *testing.T) {
	for _, c := range []struct {
		name, file string
		op         edit.Operation
		want       []string
	}{
		{"JSON key", "d/tally.json", edit.Operation{Kind: edit.OpAddEntry, Path: "tally.byPick", Key: edit.Source(`"default"`), Value: edit.Int(2)}, []string{`"default": 2`}},
		{"JSON map", "d/tally.json", edit.Operation{Kind: edit.OpSet, Path: "tally.byPick", Value: edit.Source(`{ wolf: 1, "default": 3 }`)}, []string{`"default": 3`}},
		{"JSON integer", "d/tally.json", edit.Operation{Kind: edit.OpSet, Path: "tally.amount", Value: edit.Source(`5`)}, []string{`"amount": 5`}},
		{"Canon key", "d/d.canon", edit.Operation{Kind: edit.OpAddEntry, Path: "canonTally.byPick", Key: edit.Source(`"default"`), Value: edit.Int(2)}, []string{`byPick: { "default": 2 }`}},
		{"Canon record", "d/d.canon", edit.Operation{Kind: edit.OpSet, Path: "canonTally", Value: edit.Source(`{ goal: paint, amount: "lots" }`)}, []string{`amount: "lots"`}},
	} {
		applyDepOn(t, litFS(), symCase{c.name, c.op, c.file, c.want})
		roundTrip(t, c.name, litFS(), []edit.Operation{c.op})
	}
}

// TYPES.md 11.4, API.md V1: a literal no branch of the dependent type takes is a *ValueError.
func TestSourceDependentLiteralRefused(t *testing.T) {
	s := open(t, litFS(), nil, "", "d")
	for _, op := range []edit.Operation{
		{Kind: edit.OpSet, Path: "tally.amount", Value: edit.Source(`2.5`)},
		{Kind: edit.OpAddEntry, Path: "tally.byPick", Key: edit.Source(`7`), Value: edit.Int(1)},
	} {
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
		var ve *edit.ValueError
		if !errors.As(err, &ve) {
			t.Errorf("%s %v: %v, want a *ValueError", op.Path, op.Value, err)
		}
	}
}
