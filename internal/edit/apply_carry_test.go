package edit_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// carrySrc holds, in a Never branch (DECISIONS 175), data of fields with wire rules of their own:
// a unit, int, bits and a none marker.
const carrySrc = `/// P.
package d

/// A goal.
enum Goal { kill, visit, paint }

/// A colour.
enum Color { red, green = "GREEN" }

/// A mark.
enum Mark @codes(UInt8) { a = 1, b = 2, c = 4 }

/// What a goal picks.
type Pick(g: Goal) = match g {
  kill => String
  visit => Never
  paint => Color
}

/// A tag.
record Tag {
  /// Its goal.
  goal: Goal = paint
  /// Waits by pick.
  waits: {Pick(goal): Duration} = {} @json(unit: s)
  /// Flags by pick.
  flags: {Pick(goal): Bool} = {} @json(int)
  /// Its marks.
  marks: [Mark] = [] @json(bits)
  /// Its pick, none written as a dash.
  pick: Pick(goal)? @json(none: "-")
}

/// The root.
record Root {
  /// A tag.
  tag: Tag = {}
}

/// Loaded.
let root: Root = load("root.json")
`

const carryRoot = `{
  "tag": {
    "goal": "visit",
    "waits": {
      "zzz": 5
    },
    "flags": {
      "zzz": 1
    },
    "marks": 3,
    "pick": "zzz"
  }
}
`

func carryFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(carrySrc), "law/d/root.json": file(carryRoot)}
}

// API.md E22, E23 (log-2026-09-29 M4 B10-r2): the Undo carries held data in the scope its inverse
// reads it in, the field's unit, int, bits and none marker, or an item's; a bits list, one number,
// is written whole by item operations (WIRE.md 5.3): every file comes back byte for byte.
func TestUndoCarryScope(t *testing.T) {
	set := func(path, src string) []edit.Operation {
		return []edit.Operation{{Kind: edit.OpSet, Path: path, Value: edit.Source(src)}}
	}
	reset := func(path string) []edit.Operation { return []edit.Operation{{Kind: edit.OpReset, Path: path}} }
	remove := func(path string) []edit.Operation { return []edit.Operation{{Kind: edit.OpRemove, Path: path}} }
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"set, unit", set("root.tag.waits", "{}")},
		{"set, int", set("root.tag.flags", "{}")},
		{"set, bits", set("root.tag.marks", "[c]")},
		{"set, none marker", set("root.tag.pick", "none")},
		{"reset, unit", reset("root.tag.waits")},
		{"reset, int", reset("root.tag.flags")},
		{"reset, bits", reset("root.tag.marks")},
		{"reset, none marker", reset("root.tag.pick")},
		{"retype", []edit.Operation{{Kind: edit.OpSet, Path: "root.tag.goal", Value: edit.Member("paint")}}},
		{"remove, unit", remove("root.tag.waits[zzz]")},
		{"remove, int", remove("root.tag.flags[zzz]")},
		{"remove, bits", remove("root.tag.marks[0]")},
		{"add, bits", []edit.Operation{{Kind: edit.OpAdd, Path: "root.tag.marks", Value: edit.Member("c")}}},
		{"insert, bits", []edit.Operation{{Kind: edit.OpInsert, Path: "root.tag.marks", Value: edit.Member("c")}}},
		{"move, bits", []edit.Operation{{Kind: edit.OpMove, Path: "root.tag.marks[0]", Index: 1}}},
	} {
		roundTrip(t, c.name, carryFS(), c.ops)
	}
	// A removed item's held data is carried in its collection's item scope, which its Insert or
	// AddEntry inverse reads it in (log-2026-09-29 M4 B10-r2).
	roundTrip(t, "remove, int element", itemCarryFS(), remove("tag.sets[0]"))
	roundTrip(t, "remove, unit entry", itemCarryFS(), remove(`tag.waits["a"]`))
	// A Set of an item holding a decoded symbol key below the field's own value: the key is
	// written back as read, the values in the field's unit or int (log-2026-09-29 M4 B10-r3).
	roundTrip(t, "set, unit entry", itemCarryFS(), set(`tag.waits["a"]`, "{}"))
	roundTrip(t, "set, int element", itemCarryFS(), set("tag.sets[0]", "{}"))
	roundTrip(t, "set, deeper unit entry", itemCarryFS(), set(`tag.deep["a"]["b"]`, "{}"))
	roundTrip(t, "set, deep unit entry", itemCarryFS(), set(`tag.deep["a"]`, "{}"))
	roundTrip(t, "remove, deeper unit entry", itemCarryFS(), remove(`tag.deep["a"]["b"]`))
}

// itemCarrySrc holds Never-branch data inside the items of collections whose field has a unit or int.
const itemCarrySrc = `/// P.
package d

/// A goal.
enum Goal { kill, visit }

/// What a goal picks.
type Pick(g: Goal) = match g {
  kill => String
  visit => Never
}

/// A tag.
record Tag {
  /// Its goal.
  goal: Goal = visit
  /// Flag sets.
  sets: [{Pick(goal): Bool}] = [] @json(int)
  /// Wait sets.
  waits: {String: {Pick(goal): Duration}} = {} @json(unit: s)
  /// Wait sets by group.
  deep: {String: {String: {Pick(goal): Duration}}} = {} @json(unit: s)
}

/// Loaded.
let tag: Tag = load("t.json")
`

const itemCarryJSON = `{
  "sets": [
    {
      "zzz": 1
    }
  ],
  "waits": {
    "a": {
      "zzz": 5
    }
  },
  "deep": {
    "a": {
      "b": {
        "zzz": 5
      }
    }
  }
}
`

func itemCarryFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(itemCarrySrc), "law/d/t.json": file(itemCarryJSON)}
}
