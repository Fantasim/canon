package edit_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// rootLawSrc is a table whose entries hold a field typed by an earlier one, and a table of none.
const rootLawSrc = `package d

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
  /// A note.
  note: String = ""
  /// A count.
  n: Int = 0
}

/// A plain row.
record P {
  /// A note.
  note: String = ""
  /// A count.
  n: Int = 0
}

/// A count.
let k0: Int = 4

/// Quests.
let qs: table Q = {
  a { items: true, rewards: ["s"], note: "x" }
}

/// Rows.
let ps: table P = {
  a { note: "x" }
}
`

// rootLaw is rootLawSrc with layers amending a quest beside an identifier (lz), all literal (ly),
// and a plain row beside an identifier (px).
func rootLaw() mapFS {
	return mapFS{
		"law/project.canon":    file(projectCanon),
		"law/d/d.canon":        file(rootLawSrc),
		"law/d/lz.layer.canon": file("package d\nlayer lz\n\namend qs {\n  a: { n: k0, items: true, rewards: [\"s\"], note: \"y\" }\n}\n"),
		"law/d/ly.layer.canon": file("package d\nlayer ly\n\namend qs {\n  a: { items: true, rewards: [\"s\"], note: \"y\" }\n}\n"),
		"law/d/px.layer.canon": file("package d\nlayer px\n\namend ps {\n  a: { n: k0, note: \"y\" }\n}\n"),
	}
}

// rootRow is an op under an edit layer and what Editable and Apply say of it.
type rootRow struct {
	name  string
	layer string
	in    states
	op    edit.Operation
	want  wayAnswer
}

// rootComputed is the answer of every edit through an inactive layer of a dependent root.
var rootComputed = wayAnswer{edit.ReasonComputed, edit.ReasonComputed, "", ""}

// rootRows are TestWayOutDependentRoot's cases.
var rootRows = []rootRow{
	{"beside an identifier: Set of the driver", "lz", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].items", Value: edit.Bool(false)}, rootComputed},
	{"beside an identifier: Set of a plain field", "lz", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].note", Value: edit.Str("z")}, rootComputed},
	{"beside an identifier: Set of a dependent element", "lz", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].rewards[0]", Value: edit.Str("t")}, rootComputed},
	{"all literal: Set of the driver", "ly", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].items", Value: edit.Bool(false)}, rootComputed},
	{"all literal: Set of a plain field", "ly", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].note", Value: edit.Str("z")}, rootComputed},
	{"all literal: Set of a dependent element", "ly", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].rewards[0]", Value: edit.Str("t")}, rootComputed},
	{"a new line of the driver", "zz", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].items", Value: edit.Bool(false)}, rootComputed},
	{"a new line of a plain field", "zz", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].note", Value: edit.Str("z")}, rootComputed},
	// Active, nothing changes; the driver beside an identifier is EDIT-LAYERPARITY's split, pinned as it is.
	{"beside an identifier, active: Set of the driver", "lz", inActive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].items", Value: edit.Bool(false)}, wayAnswer{edit.ReasonNone, edit.ReasonComputed, "", ""}},
	{"beside an identifier, active: Set of a plain field", "lz", inActive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].note", Value: edit.Str("z")}, wayAnswer{}},
	{"beside an identifier, active: Set of a dependent element", "lz", inActive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].rewards[0]", Value: edit.Str("t")}, wayAnswer{}},
	{"all literal, active: Set of the driver", "ly", inActive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].items", Value: edit.Bool(false)}, wayAnswer{}},
	{"all literal, active: Set of a plain field", "ly", inActive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].note", Value: edit.Str("z")}, wayAnswer{}},
	{"all literal, active: Set of a dependent element", "ly", inActive, edit.Operation{Kind: edit.OpSet, Path: "d:qs[a].rewards[0]", Value: edit.Str("t")}, wayAnswer{}},
	{"no dependent type: Set of a plain field beside an identifier", "px", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:ps[a].note", Value: edit.Str("z")}, wayAnswer{}},
	{"no dependent type: Set of the identifier", "px", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:ps[a].n", Value: edit.Int(5)}, wayAnswer{edit.ReasonComputed, edit.ReasonComputed, edit.WayLiteral, edit.WayLiteral}},
	{"no dependent type: a new line", "zz", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:ps[a].note", Value: edit.Str("z")}, wayAnswer{}},
}

// API.md W11, V1 (DECISIONS 273, 462): with the edit layer inactive, every edit through it of a root
// whose type holds a dependent type is `computed`, no way out, as no driver of the layer is seen;
// a root of none keeps its verdicts.
func TestWayOutDependentRoot(t *testing.T) {
	fsys := rootLaw()
	for _, c := range rootRows {
		for _, layers := range c.in.sessions(c.layer) {
			t.Run(c.name, func(t *testing.T) {
				if got := askWay(t, fsys, layers, c.layer, c.op); got != c.want {
					t.Errorf("layers %v: %s: %+v, want %+v", layers, c.name, got, c.want)
				}
			})
		}
	}
}
