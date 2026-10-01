package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
)

// lootSrc types values by an earlier field (SPEC 5.11): list elements, map values, the elements
// of a list in a variant case, a nested application, and a record typed by its argument; each
// entry writes one kind, so a request that changes its driver and its values ends error-free.
const lootSrc = `package d

/// What a quest counts.
enum Goal { kill, collect }

/// A reward: gold, or an item.
type Reward(item: Bool) = match item {
  false => Int(1..)
  true => String(1..)
}

/// What a quest's loot is, by goal: a reward, or a count.
type Loot(g: Goal, item: Bool) = match g {
  kill => Reward(item)
  collect => Int(1..)
}

/// Rewards, typed by the pack's argument.
record Pack(item: Bool) {
  /// What it gives.
  rewards: [Reward(item)] = []
  /// Its best reward.
  best: Reward(item)?
}

/// What a step does.
variant Act {
  /// Hunts.
  hunt {
    /// Whether its rewards are items.
    items: Bool = false
    /// What it gives.
    rewards: [Reward(items)] = []
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
  goal: Goal = kill
  /// Whether its rewards are items rather than gold.
  items: Bool = false
  /// What completing it gives.
  rewards: [Reward(items)] = []
  /// What its rewards cost.
  prices: {String: Reward(items)} = {}
  /// What it drops.
  loot: [Loot(goal, items)] = []
  /// A pack typed by items.
  pack: Pack(items)?
  /// Its step.
  act: Act = wait {}
}

/// The quests.
let quests: table Quest = {
  gather { items: true, rewards: ["a"] }
  trade { items: true, prices: { "x": "p" } }
  raid { items: true, loot: ["l"] }
  stash { items: true, pack: { rewards: ["q"], best: "z" } }
  hunt { act: hunt { items: true, rewards: ["h"] } }
  cache { items: true, pack: {} }
}

/// A quest a JSON source states.
let loaded: Quest = load("loaded.json")
`

const lootJSON = `{
  "items": true,
  "rewards": ["a"],
  "prices": {
    "x": "p"
  },
  "pack": {}
}
`

func lootFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(lootSrc), "law/d/loaded.json": file(lootJSON)}
}

// API.md E1, V1, E15: a value whose type a field computes is typed against the branch the earlier
// operations select; the request applies, each write minimal (M6), the result checks clean, and
// its Undo, then the Undo of that Undo, give back the values before, then after (E22).
func TestE1DependentAfterDriver(t *testing.T) {
	no, yes := edit.Bool(false), edit.Bool(true)
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"list element", []edit.Operation{setAt("quests.gather.items", no), setAt("quests.gather.rewards[0]", edit.Int(3))}},
		{"list element, by position", []edit.Operation{setAt("quests.gather.items", no), setAt("quests.gather.rewards[#0]", edit.Int(3))}},
		{"map value", []edit.Operation{setAt("quests.trade.items", no), setAt(`quests.trade.prices["x"]`, edit.Int(3))}},
		{"list element in a case", []edit.Operation{setAt("quests.hunt.act.items", no), setAt("quests.hunt.act.rewards[0]", edit.Int(3))}},
		{"list element in a case, after a SetCase", []edit.Operation{
			{Kind: edit.OpSetCase, Path: "quests.gather.act", Case: "hunt", Value: edit.Obj{"rewards": edit.Source("[5]")}},
			setAt("quests.gather.act.items", yes), setAt("quests.gather.act.rewards[0]", edit.Str("h")),
		}},
		{"nested application, inner driver", []edit.Operation{setAt("quests.raid.items", no), setAt("quests.raid.loot[0]", edit.Int(3))}},
		{"nested application, outer driver", []edit.Operation{setAt("quests.raid.goal", edit.Member("collect")), setAt("quests.raid.loot[0]", edit.Int(7))}},
		{"driver through an argument, element", []edit.Operation{
			setAt("quests.stash.items", no), setAt("quests.stash.pack.rewards[0]", edit.Int(3)), setAt("quests.stash.pack.best", edit.Int(4)),
		}},
		{"driver changed and changed back", []edit.Operation{
			setAt("quests.gather.items", no), setAt("quests.gather.rewards[0]", edit.Int(3)),
			setAt("quests.gather.items", yes), setAt("quests.gather.rewards[0]", edit.Str("b")),
		}},
		{"driver through an argument, field left out", []edit.Operation{setAt("quests.cache.pack.best", edit.Str("z"))}},
		{"driver through an argument, field left out, after the driver", []edit.Operation{
			setAt("quests.cache.items", no), setAt("quests.cache.pack.best", edit.Int(4)),
		}},
		{"JSON source, driver through an argument", []edit.Operation{setAt("loaded.pack.best", edit.Str("z"))}},
		{"JSON source, map value", []edit.Operation{setAt(`loaded.prices["x"]`, edit.Str("q"))}},
	} {
		checkClean(t, c.name, lootFS(), c.ops)
		undoTwice(t, c.name, lootFS(), c.ops)
	}
}

// API.md E1, V1 (8.3 Add, Insert, AddEntry): an item added to a collection whose item type a
// field computes is typed against the branch the earlier operations select, or the one the
// record selects when none changed it; each write is minimal (M6) and the Undo round-trips (E22).
func TestE1DependentItemAdded(t *testing.T) {
	no := edit.Bool(false)
	add := func(path string, v edit.Lit) edit.Operation {
		return edit.Operation{Kind: edit.OpAdd, Path: path, Value: v}
	}
	entry := func(path, key string, v edit.Lit) edit.Operation {
		return edit.Operation{Kind: edit.OpAddEntry, Path: path, Key: edit.Key(key), Value: v}
	}
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"Add", []edit.Operation{add("quests.gather.rewards", edit.Str("b"))}},
		{"Insert, after the driver", []edit.Operation{
			setAt("quests.gather.items", no), setAt("quests.gather.rewards[0]", edit.Int(3)),
			{Kind: edit.OpInsert, Path: "quests.gather.rewards", Index: 0, Value: edit.Int(2)},
		}},
		{"AddEntry", []edit.Operation{entry("quests.trade.prices", "y", edit.Str("b"))}},
		{"Add to a list left to its default, driver through an argument", []edit.Operation{add("quests.cache.pack.rewards", edit.Str("b"))}},
		{"JSON source, AddEntry", []edit.Operation{entry("loaded.prices", "y", edit.Str("b"))}},
		{"Add, after the driver", []edit.Operation{
			setAt("quests.gather.items", no), setAt("quests.gather.rewards[0]", edit.Int(3)), add("quests.gather.rewards", edit.Int(4)),
		}},
		{"AddEntry, after the driver", []edit.Operation{
			setAt("quests.trade.items", no), setAt(`quests.trade.prices["x"]`, edit.Int(3)), entry("quests.trade.prices", "y", edit.Int(4)),
		}},
	} {
		checkClean(t, c.name, lootFS(), c.ops)
		undoTwice(t, c.name, lootFS(), c.ops)
	}
	s := open(t, lootFS(), nil, "", "d")
	ops := []edit.Operation{setAt("quests.gather.items", no), add("quests.gather.rewards", edit.Str("b"))}
	var ve *edit.ValueError
	if _, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops}); !errors.As(err, &ve) {
		t.Errorf("Add of a String after the driver selects Int: %v, want a ValueError", err)
	}
}

// API.md E1, V1, E14: a SetCase to the current case changes the driver of a list in it; the
// element Set after it is typed against the branch the SetCase selected and the request applies,
// each write minimal (M6), the result checking clean.
func TestE1DependentDriverBySetCase(t *testing.T) {
	ops := []edit.Operation{
		{Kind: edit.OpSetCase, Path: "quests.hunt.act", Case: "hunt", Value: edit.Obj{"items": edit.Bool(false)}},
		setAt("quests.hunt.act.rewards[0]", edit.Int(3)),
	}
	checkClean(t, "driver set by SetCase", lootFS(), ops)
	undoTwice(t, "driver set by SetCase", lootFS(), ops)
}

// API.md E1, V1: the branch the earlier operations select also refuses what it does not take,
// a driver changed back included; the element alone, its driver unchanged, is typed the same.
func TestE1DependentRefused(t *testing.T) {
	no, yes := edit.Bool(false), edit.Bool(true)
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"list element", []edit.Operation{setAt("quests.gather.items", no), setAt("quests.gather.rewards[0]", edit.Str("b"))}},
		{"changed back", []edit.Operation{
			setAt("quests.gather.items", no), setAt("quests.gather.items", yes), setAt("quests.gather.rewards[0]", edit.Int(3)),
		}},
		{"map value", []edit.Operation{setAt("quests.trade.items", no), setAt(`quests.trade.prices["x"]`, edit.Str("b"))}},
		{"nested, outer driver", []edit.Operation{setAt("quests.raid.goal", edit.Member("collect")), setAt("quests.raid.loot[0]", edit.Str("b"))}},
		{"through an argument", []edit.Operation{setAt("quests.stash.items", no), setAt("quests.stash.pack.best", edit.Str("b"))}},
		{"unchanged driver", []edit.Operation{setAt("quests.gather.rewards[0]", edit.Int(3))}},
	} {
		s := open(t, lootFS(), nil, "", "d")
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		var oe *edit.OpError
		var ve *edit.ValueError
		if !errors.As(err, &oe) || oe.Index != len(c.ops)-1 || !errors.As(err, &ve) {
			t.Errorf("%s: %v, want a ValueError at operation %d", c.name, err, len(c.ops)-1)
		}
	}
}

// checkClean applies ops to fsys and checks the state they leave: no error finding (E18).
func checkClean(t *testing.T, name string, fsys mapFS, ops []edit.Operation) {
	t.Helper()
	edited, ok := applied(t, name, fsys, ops)
	if !ok {
		return
	}
	s := open(t, edited.fsys, nil, "", "d")
	values(s, false) // verification runs as the values are forced
	for _, f := range s.a.Bag("d").Findings() {
		if f.Severity == diag.Error {
			t.Errorf("%s: after the edit: %s %s %s", name, f.Code, f.Path, f.Message)
		}
	}
}
