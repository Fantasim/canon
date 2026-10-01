package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// tableLayerSrc is a table an edit layer amends, and a template a line spreads.
const tableLayerSrc = `package d

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

/// A template.
let tmpl: Quest = { goal: kill, target: "wolf", note: "t" }

/// The quests.
let quests: table Quest = {
  slay { goal: collect, target: 10 }
  gather { goal: collect, target: 5 }
}
`

const devLayerFile = "law/d/dev.layer.canon"

func tableLayerFS(dev string) mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(tableLayerSrc), devLayerFile: file(dev)}
}

func layerDev(lines string) string {
	return "package d\nlayer dev\n\namend quests {\n" + lines + "}\n"
}

// opsRefused reports Apply refusing ops in view v with a NotEditableError of reason computed.
func opsRefused(t *testing.T, fs mapFS, v undoView, ops []edit.Operation) bool {
	t.Helper()
	s := open(t, fs, v.layers, v.edit, "d")
	_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
	var ne *edit.NotEditableError
	return errors.As(err, &ne) && ne.Reason == edit.ReasonComputed
}

// API.md W11, E22, E23 (log-2026-10-01 M4.1 rulings c, d): an AddEntry through an edit layer that
// already amends the table adds one line and keeps every other; its Undo gives the file back.
func TestLayerAddEntryKeepsLines(t *testing.T) {
	dev := layerDev("  slay.note: \"x\"\n  hunt: { goal: kill, target: \"x\" }\n")
	add := edit.Operation{Kind: edit.OpAddEntry, Path: "quests", Key: edit.Key("fresh"), Value: edit.Source(`{ goal: kill, target: "f" }`)}
	for _, layers := range [][]string{{"dev"}} {
		undoIn(t, undoSpec{name: "AddEntry", fsys: tableLayerFS(dev), view: undoView{layers, "dev"}, ops: []edit.Operation{add},
			views: [][]string{{"dev"}, nil}, files: []string{devLayerFile}})
		_, f1, ok := applyIn(t, "AddEntry", tableLayerFS(dev), undoView{layers, "dev"}, []edit.Operation{add})
		if want := layerDev("  slay.note: \"x\"\n  hunt: { goal: kill, target: \"x\" }\n  fresh: { goal: kill, target: \"f\" }\n"); ok && string(f1[devLayerFile].Data) != want {
			t.Errorf("%v: AddEntry writes\n%s\nwant\n%s", layers, f1[devLayerFile].Data, want)
		}
	}
	s := open(t, tableLayerFS(dev), nil, "dev", "d")
	_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{add}})
	var ne *edit.NotEditableError
	if !errors.As(err, &ne) || ne.Reason != edit.ReasonLayer {
		t.Errorf("AddEntry through an inactive layer, whose entry no path reaches: %v, want NotEditable (layer)", err)
	}
}

// API.md E22, E23, W5 (log-2026-10-01 M4.1 ruling c): removing an entry the edit layer creates,
// before another it creates, is undone with no Move, the entries coming back in their order; with
// a later entry line no Source writes (a spread), the Remove is refused (computed).
func TestLayerRemoveEntryOrder(t *testing.T) {
	dev := layerDev("  hunt: { goal: kill, target: \"x\" }\n  camp: { goal: kill, target: \"c\" }\n")
	remove := []edit.Operation{{Kind: edit.OpRemove, Path: "quests.hunt"}}
	for _, layers := range [][]string{{"dev"}} {
		undoIn(t, undoSpec{name: "Remove", fsys: tableLayerFS(dev), view: undoView{layers, "dev"}, ops: remove,
			views: [][]string{{"dev"}, nil}, files: []string{devLayerFile}})
		undoIn(t, undoSpec{name: "Remove, dependent", fsys: tableLayerFS(dev), view: undoView{layers, "dev"}, views: [][]string{{"dev"}, nil},
			files: []string{devLayerFile}, ops: append(remove, setAt("quests.slay.goal", edit.Member("kill")), setAt("quests.slay.target", edit.Str("s")))})
	}
	spread := layerDev("  hunt: { goal: kill, target: \"x\" }\n  camp: { ...tmpl, note: \"c\" }\n")
	if !opsRefused(t, tableLayerFS(spread), undoView{[]string{"dev"}, "dev"}, remove) {
		t.Errorf("Remove before a spread line: not refused as computed")
	}
}

// API.md W5, W11, E22 (log-2026-10-01 M4.1 rulings): under an edit layer every edit's Undo is the
// layer's own lines, one operation too: a whole-record Set replacing a spread or computed line is
// refused (computed); one replacing literal lines gives them back, a plain Set the file exactly.
func TestLayerSingleOpUndo(t *testing.T) {
	collect := edit.Member("collect")
	whole := []edit.Operation{setAt("quests.slay", edit.Source(`{ goal: collect, target: 3 }`))}
	for _, layers := range [][]string{{"dev"}, nil} {
		v := undoView{layers, "dev"}
		for _, line := range []string{"  slay: { ...tmpl, target: \"boar\" }\n", "  slay.goal: kill\n  slay.note: tag\n"} {
			src := tableLayerSrc + "\n/// A tag.\nlet tag: String = \"t\"\n"
			fs := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(src), devLayerFile: file(layerDev(line))}
			if !opsRefused(t, fs, v, whole) {
				t.Errorf("%v %q: a single Set replacing a line no Source writes: not refused as computed", layers, line)
			}
		}
		literal := layerDev("  slay.goal: kill\n  slay.target: \"boar\"\n")
		undoIn(t, undoSpec{name: "whole record", fsys: tableLayerFS(literal), view: v, ops: whole, views: [][]string{{"dev"}, nil}, lines: []string{devLayerFile}})
		for _, ops := range [][]edit.Operation{{setAt("quests.slay.goal", collect)}, {setAt("quests.gather.note", edit.Str("n"))}} {
			undoIn(t, undoSpec{name: "single op", fsys: tableLayerFS(literal), view: v, ops: ops, views: [][]string{{"dev"}, nil}, files: []string{devLayerFile}})
		}
	}
}

// API.md W5, W11, E22 (log-2026-10-01 M4.1 rulings): a line spreading a template types as no
// Source, so a request whose Undo must write it back is refused (computed), not failed inside.
func TestLayerSpreadLineRefused(t *testing.T) {
	fs := tableLayerFS(layerDev("  slay: { ...tmpl, target: \"boar\" }\n"))
	collect := edit.Member("collect")
	for _, ops := range [][]edit.Operation{
		{setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25))},
		{setAt("quests.slay.target", edit.Str("elk")), setAt("quests.slay.goal", collect)},
	} {
		if !opsRefused(t, fs, undoView{[]string{"dev"}, "dev"}, ops) {
			t.Errorf("%+v: not refused as computed", ops)
		}
	}
}

// API.md E22 (log-2026-10-01 M4.1 ruling b): two lines' right-hand sides are compared token by
// token: spacing aside, a string literal's own spaces count.
func TestLayerLineTextTokens(t *testing.T) {
	if edit.LineText(`"a b"`) == edit.LineText(`"ab"`) {
		t.Errorf(`"a b" and "ab" compare equal`)
	}
	if edit.LineText(`{ goal: kill,  target: "x" }`) != edit.LineText(`{goal: kill, target: "x"}`) {
		t.Errorf("spacing alone makes two right-hand sides differ")
	}
}

// API.md E22, W11 (log-2026-10-01 M4.1 rulings): random requests through an edit layer below
// another, its lines plain, creating entries, or spreading a template: none fails inside, every
// Undo applies and gives every value back in each view.
func TestUndoTwoLayersRandom(t *testing.T) {
	m := func(s string) edit.Lit { return edit.Member(s) }
	pool := []edit.Operation{
		setAt("quests.slay.goal", m("collect")), setAt("quests.slay.goal", m("kill")),
		setAt("quests.slay.target", edit.Int(25)), setAt("quests.slay.target", edit.Str("boar")),
		{Kind: edit.OpReset, Path: "quests.slay.target"}, {Kind: edit.OpReset, Path: "quests.slay.goal"},
		setAt("quests.gather.goal", m("kill")), setAt("quests.gather.target", edit.Str("x")), setAt("quests.gather.target", edit.Int(8)),
		setAt("quests.hunt.goal", m("collect")), setAt("quests.hunt.target", edit.Int(4)), setAt("quests.hunt.target", edit.Str("y")),
		{Kind: edit.OpRemove, Path: "quests.hunt"},
		{Kind: edit.OpAddEntry, Path: "quests", Key: edit.Key("hunt"), Value: edit.Source(`{ goal: collect, target: 2 }`)},
		setAt("tmpl.goal", m("collect")), setAt("tmpl.target", edit.Int(6)), setAt("quests.camp.target", edit.Int(9)), setAt("quests.camp.goal", m("collect")),
	}
	dev := layerDev("  slay.goal: kill\n  slay.target: \"boar\"\n  hunt: { goal: kill, target: \"x\" }\n  camp: { ...tmpl, note: \"c\" }\n")
	top := "package d\nlayer top\n\namend quests {\n  gather.note: \"top\"\n}\n"
	next := lcg(777)
	for range randomUndos {
		fs := tableLayerFS(dev)
		fs["law/d/top.layer.canon"] = file(top)
		randomUndo(t, fs, undoView{[]string{"dev", "top"}, "dev"}, pickOps(next, pool), [][]string{{"dev", "top"}, {"dev"}, nil})
	}
}
