package edit_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// shiftSrc has a map, a plain list, a keyed list and a table that dev addresses by position, a
// keyed list dev amends by key, and lists holding a let dev amends, by name and through a spread.
const shiftSrc = `package d

/// An item.
record Item {
  /// Its id.
  id: String
  /// A count.
  n: Int = 0
}

/// An entry.
record E {
  /// A value.
  v: Int = 0
}

/// A box.
record Box {
  /// Weights.
  m: {String: Int} = {}
  /// Numbers.
  l: [Int] = []
  /// Items.
  k: [Item] keyed by id = []
  /// Rows.
  r: table E = {}
}

/// The box.
let box: Box = {
  m: { "a": 1, "b": 2, "c": 3 }
  l: [1, 2, 3]
  k: [{ id: "a" }, { id: "b" }, { id: "c" }]
  r: {
    a { v: 1 }
    b { v: 2 }
    c { v: 3 }
  }
}

/// Items by key.
let ks: [Item] keyed by id = [{ id: "a" }, { id: "b" }]

/// A shared entry.
let sb: E = { v: 1 }

/// Its list.
let pi: [E] = [sb, { v: 2 }]

/// Its list, spread.
let ps: [E] = [{ ...sb }, { v: 2 }]
`

const shiftDev = "package d\nlayer dev\n\namend box {\n  m[#2]: 30\n  l[2]: 30\n  k[#2].n: 30\n  r[#2].v: 30\n}\n\n" +
	"amend ks {\n  a.n: 7\n}\n\namend sb {\n  v: 5\n}\n"

const shiftFile = "law/d/d.canon"

func shiftFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), shiftFile: file(shiftSrc), "law/d/dev.layer.canon": file(shiftDev)}
}

// API.md E22, W10 (EDIT-MATERIALIZE ruling): with dev active, a base Move in a collection a
// line of dev names by position, a map, a plain list, a keyed list or a table, is `layered` for
// Editable and Apply alike, dev's lines and not the values deciding it.
func TestShiftMoveByPosition(t *testing.T) {
	for _, path := range []string{"d:box.m[a]", "d:box.l[0]", "d:box.k[a]", "d:box.r[a]"} {
		ed, ap, _ := guardVerdict(t, shiftFS(), edit.Operation{Kind: edit.OpMove, Path: path, Index: 2})
		if ed != edit.ReasonLayered || ap != edit.ReasonLayered {
			t.Errorf("Move %s to 2 under dev: Editable %v, Apply %v, want both layered", path, ed, ap)
		}
	}
}

// API.md E9, E22, E23 (EDIT-MATERIALIZE ruling): a base Move dev's lines name by no position
// applies, whatever value dev amends below or beside it: a keyed list amended by key, and lists
// holding a let dev amends, by name and through a spread; each Undo gives every value back.
func TestShiftMoveByName(t *testing.T) {
	for _, path := range []string{"d:ks[b]", "d:pi[1]", "d:ps[1]"} {
		op := edit.Operation{Kind: edit.OpMove, Path: path, Index: 0}
		if ed, ap, _ := guardVerdict(t, shiftFS(), op); ed != edit.ReasonNone || ap != edit.ReasonNone {
			t.Errorf("Move %s to 0 under dev: Editable %v, Apply %v, want both editable", path, ed, ap)
			continue
		}
		undoIn(t, undoSpec{name: "Move " + path, fsys: shiftFS(), view: undoView{[]string{"dev"}, ""}, ops: []edit.Operation{op},
			views: [][]string{{"dev"}, nil}, files: []string{shiftFile}})
	}
}
