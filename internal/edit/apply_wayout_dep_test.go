package edit_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// depLaw is lootSrc with a goal a let states, and edit layers whose lines give dependent fields.
func depLaw() mapFS {
	fsys := lootFS()
	fsys["law/d/d.canon"] = file(lootSrc + "\n/// A goal.\nlet g0: Goal = collect\n")
	lines := map[string]string{
		"lq": "gather: { goal: g0, items: false, rewards: [3] }",
		"lt": "trade: { goal: g0, items: false, prices: { \"x\": 3 } }",
		"lr": "raid: { goal: g0, items: false, loot: [3] }",
		"la": "gather: { items: false, rewards: [3] }",
	}
	for layer, line := range lines { //canon:unordered fills a map
		fsys["law/d/"+layer+".layer.canon"] = file("package d\nlayer " + layer + "\n\namend quests {\n  " + line + "\n}\n")
	}
	return fsys
}

// depRow is an op under an edit layer giving a dependent field, and the reason both judges give.
type depRow struct {
	name   string
	layer  string
	in     states
	op     edit.Operation
	reason edit.Reason
}

// depRows are TestWayOutDependentInactive's cases.
var depRows = []depRow{
	{"list element by its driver beside an expression", "lq", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:quests.gather.rewards[0]", Value: edit.Str("b")}, edit.ReasonComputed},
	{"map value by its driver beside an expression", "lt", inInactive, edit.Operation{Kind: edit.OpSet, Path: `d:quests.trade.prices["x"]`, Value: edit.Str("b")}, edit.ReasonComputed},
	{"nested application beside an expression", "lr", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:quests.raid.loot[0]", Value: edit.Str("b")}, edit.ReasonComputed},
	{"list element by its driver in an all-literal line", "la", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:quests.gather.rewards[0]", Value: edit.Str("b")}, edit.ReasonComputed},
	{"a field of the all-literal line, below a dependent type", "la", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:quests.gather.goal", Value: edit.Member("kill")}, edit.ReasonComputed},
	{"a non-dependent sibling beside an expression, a dependent root", "lq", inInactive, edit.Operation{Kind: edit.OpSet, Path: "d:quests.gather.items", Value: edit.Bool(true)}, edit.ReasonComputed},
	{"list element, the layer active", "lq", inActive, edit.Operation{Kind: edit.OpSet, Path: "d:quests.gather.rewards[0]", Value: edit.Int(5)}, edit.ReasonNone},
	{"map value, the layer active", "lt", inActive, edit.Operation{Kind: edit.OpSet, Path: `d:quests.trade.prices["x"]`, Value: edit.Int(5)}, edit.ReasonNone},
	{"nested application, the layer active", "lr", inActive, edit.Operation{Kind: edit.OpSet, Path: "d:quests.raid.loot[0]", Value: edit.Int(5)}, edit.ReasonNone},
	{"all-literal line, the layer active", "la", inActive, edit.Operation{Kind: edit.OpSet, Path: "d:quests.gather.rewards[0]", Value: edit.Int(5)}, edit.ReasonNone},
}

// API.md W11, V1 (DECISIONS 273, 462): with the edit layer inactive, every edit of a root whose type
// holds a dependent type is `computed`, no way out, as no driver of the layer is seen; with the
// layer active nothing changes. Editable and Apply agree.
func TestWayOutDependentInactive(t *testing.T) {
	fsys := depLaw()
	for _, c := range depRows {
		for _, layers := range c.in.sessions(c.layer) {
			t.Run(c.name, func(t *testing.T) {
				got := askWay(t, fsys, layers, c.layer, c.op)
				if want := (wayAnswer{c.reason, c.reason, "", ""}); got != want {
					t.Errorf("layers %v: %s: %+v, want %d and no way out from both", layers, c.name, got, c.reason)
				}
			})
		}
	}
}
