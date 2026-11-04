package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// depBaseSrc has a dependent list and field whose driver, items, a dev layer amends: false in the
// base, so a reward is an Int there, true under dev, a String.
const depBaseSrc = `package d

/// A reward: gold, or an item.
type Reward(item: Bool) = match item {
  false => Int(1..)
  true => String(1..)
}

/// A quest.
record Q {
  /// Whether its rewards are items.
  items: Bool = false
  /// What it gives.
  rewards: [Reward(items)] = []
  /// Its best reward.
  best: Reward(items)?
}

/// The quest.
let q: Q = { items: false }

/// The quests.
let qs: table Q = {
  a { items: false }
}
`

const depBaseDev = "package d\nlayer dev\n\namend q {\n  items: true\n}\n\namend qs {\n  a.items: true\n}\n"

// depBaseFS is depBaseSrc, with its dev layer when layered.
func depBaseFS(layered bool) mapFS {
	fs := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(depBaseSrc)}
	if layered {
		fs["law/d/dev.layer.canon"] = file(depBaseDev)
	}
	return fs
}

// API.md V1, W8 (EDIT-MATERIALIZE ruling): a base edit types its value against the declared type in
// the base's frame, what the base file holds: with dev active, as with no layer, an Int reward
// goes in, at the let and in a table entry, and a String is a *ValueError; dev's arm is the re-check's.
func TestBaseDependentTypedInBase(t *testing.T) {
	ops := []struct {
		op   edit.Operation
		fits bool
	}{
		{edit.Operation{Kind: edit.OpAdd, Path: "d:q.rewards", Value: edit.Int(2)}, true},
		{edit.Operation{Kind: edit.OpAdd, Path: "d:q.rewards", Value: edit.Str("t")}, false},
		{edit.Operation{Kind: edit.OpSet, Path: "d:q.best", Value: edit.Int(2)}, true},
		{edit.Operation{Kind: edit.OpSet, Path: "d:q.best", Value: edit.Str("t")}, false},
		{edit.Operation{Kind: edit.OpAdd, Path: "d:qs.a.rewards", Value: edit.Int(2)}, true},
		{edit.Operation{Kind: edit.OpAdd, Path: "d:qs.a.rewards", Value: edit.Str("t")}, false},
		{edit.Operation{Kind: edit.OpSet, Path: "d:qs.a.best", Value: edit.Int(2)}, true},
	}
	for _, layers := range [][]string{{"dev"}, nil} {
		for _, c := range ops {
			s := open(t, depBaseFS(len(layers) > 0), layers, "", "d")
			_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
			var ve *edit.ValueError
			switch {
			case c.fits && err != nil:
				t.Errorf("layers %v: %d %s %v: %v, want it applied", layers, c.op.Kind, c.op.Path, c.op.Value, err)
			case !c.fits && !errors.As(err, &ve):
				t.Errorf("layers %v: %d %s %v: %v, want a *ValueError against the base's Int arm", layers, c.op.Kind, c.op.Path, c.op.Value, err)
			}
		}
	}
}
