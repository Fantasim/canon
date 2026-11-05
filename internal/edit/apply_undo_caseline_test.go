package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// caseLineSrc is a box a layer amends below each field: hb at sh.r, ho at osh.r, hl at ls[1], sib at
// the sibling label only; ring keeps r (E14), square has none.
const caseLineSrc = `package d

/// A shape.
variant Shape {
  /// A circle.
  circle {
    /// Radius.
    r: Int = 1
    /// Tags.
    tags: [String] = []
  }
  /// A ring.
  ring {
    /// Radius.
    r: Int = 1
  }
  /// A square.
  square {
    /// Side.
    side: Int = 1
  }
}

/// A box.
record Box {
  /// Shape.
  sh: Shape = circle {}
  /// Optional shape.
  osh: Shape?
  /// Numbers.
  ls: [Int] = []
  /// Label.
  label: String = ""
}

/// The box.
let bb: Box = { sh: circle { r: 2 }, osh: circle { r: 3 }, ls: [1, 2], label: "b" }
`

const caseLineFile = "law/d/d.canon"

func caseLineFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), caseLineFile: file(caseLineSrc),
		"law/d/hb.layer.canon":  file("package d\nlayer hb\n\namend bb {\n  sh.r: 9\n}\n"),
		"law/d/ho.layer.canon":  file("package d\nlayer ho\n\namend bb {\n  osh.r: 8\n}\n"),
		"law/d/hl.layer.canon":  file("package d\nlayer hl\n\namend bb {\n  ls[1]: 9\n}\n"),
		"law/d/sib.layer.canon": file("package d\nlayer sib\n\namend bb {\n  label: \"s\"\n}\n"),
	}
}

func caseTo(path, c string, v edit.Obj) edit.Operation {
	return edit.Operation{Kind: edit.OpSetCase, Path: path, Case: c, Value: v}
}

// caseLineRefused is a base edit, with layers active, that leaves an active layer's line below its
// path in error (E1905: the new value lacks the field or index), and the layer its Undo meets.
type caseLineRefused struct {
	name   string
	layers []string
	op     edit.Operation
	layer  string
}

var caseLineRefusals = []caseLineRefused{
	{"SetCase to a case without the amended field", []string{"hb"}, caseTo("d:bb.sh", "square", edit.Obj{"side": edit.Int(4)}), "hb"},
	{"SetCase beside a sibling layer", []string{"hb", "sib"}, caseTo("d:bb.sh", "square", nil), "hb"},
	{"SetCase of an optional variant", []string{"ho"}, caseTo("d:bb.osh", "square", nil), "ho"},
	{"Set of the variant to another case", []string{"hb"}, edit.Operation{Kind: edit.OpSet, Path: "d:bb.sh", Value: edit.Source("square { side: 4 }")}, "hb"},
	{"Set of an optional variant to none", []string{"ho"}, edit.Operation{Kind: edit.OpSet, Path: "d:bb.osh", Value: edit.None{}}, "ho"},
	{"Remove before the amended index", []string{"hl"}, edit.Operation{Kind: edit.OpRemove, Path: "d:bb.ls[0]"}, "hl"},
	{"Set of a list shorter than the amended index", []string{"hl"}, edit.Operation{Kind: edit.OpSet, Path: "d:bb.ls", Value: edit.Source("[5]")}, "hl"},
}

// DECISIONS 466, 273, API.md E22, E23, W10: with AllowErrors, a base edit leaving an active layer's
// line below its path in error has an Undo the line refuses, so Apply refuses it `layered`.
func TestCaseLineRefused(t *testing.T) {
	for _, c := range caseLineRefusals {
		s := open(t, caseLineFS(), c.layers, "", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}, AllowErrors: true})
		var ne *edit.NotEditableError
		var oe *edit.OpError
		if !errors.As(err, &ne) || !errors.As(err, &oe) || ne.Reason != edit.ReasonLayered || ne.Layer != c.layer || oe.Index != 0 || plan != nil {
			t.Errorf("%s under %v: plan %v, %v; want refused layered by %s at operation 0, nothing written", c.name, c.layers, plan != nil, err, c.layer)
		}
	}
}

// DECISIONS 466, API.md E14, E22, E23, W10: an edit keeping every active layer's line computable
// (no layer, a sibling layer, the same case, a case keeping r, an edit layer) applies; its Undo and
// the Undo's Undo give every value and file back, in the session's layers and with none.
func TestCaseLineAccepted(t *testing.T) {
	cases := []struct {
		name string
		view undoView
		op   edit.Operation
	}{
		{"no layer", undoView{}, caseTo("d:bb.sh", "square", edit.Obj{"side": edit.Int(4)})},
		{"no layer, optional", undoView{}, caseTo("d:bb.osh", "square", nil)},
		{"no layer, Remove", undoView{}, edit.Operation{Kind: edit.OpRemove, Path: "d:bb.ls[0]"}},
		{"a sibling layer", undoView{[]string{"sib"}, ""}, caseTo("d:bb.sh", "square", edit.Obj{"side": edit.Int(4)})},
		{"the same case", undoView{[]string{"hb"}, ""}, caseTo("d:bb.sh", "circle", edit.Obj{"tags": edit.Source(`["z"]`)})},
		{"the same case, optional", undoView{[]string{"ho"}, ""}, caseTo("d:bb.osh", "circle", edit.Obj{"tags": edit.Source(`["z"]`)})},
		{"a case keeping r", undoView{[]string{"hb"}, ""}, caseTo("d:bb.sh", "ring", nil)},
		{"an Add past the amended index", undoView{[]string{"hl"}, ""}, edit.Operation{Kind: edit.OpAdd, Path: "d:bb.ls", Value: edit.Int(3)}},
		{"edit layer hb, its line", undoView{[]string{"hb"}, "hb"}, edit.Operation{Kind: edit.OpSet, Path: "d:bb.sh.r", Value: edit.Int(7)}},
		{"edit layer sib, beside hb", undoView{[]string{"hb", "sib"}, "sib"}, edit.Operation{Kind: edit.OpSet, Path: "d:bb.label", Value: edit.Str("t")}},
	}
	for _, c := range cases {
		files := []string{caseLineFile}
		if c.view.edit != "" {
			files = append(files, "law/d/"+c.view.edit+".layer.canon")
		}
		undoIn(t, undoSpec{name: c.name, fsys: caseLineFS(), view: c.view, ops: []edit.Operation{c.op},
			views: [][]string{c.view.layers, nil}, files: files})
	}
}

// poisonSrc is a table whose entries need hp; hm amends wolf.
const poisonSrc = "package d\n\n/// A monster.\nrecord M {\n  /// Hit points.\n  hp: Int\n}\n\n/// The monsters.\nlet ms: table M = {\n  wolf { hp: 1 }\n}\n"

// recordPoisonSrc is a record let whose In needs hp.
const recordPoisonSrc = "package d\n\n/// In.\nrecord In {\n  /// Hit points.\n  hp: Int\n}\n\n/// Out.\nrecord Out {\n  /// A.\n  a: In = { hp: 1 }\n}\n\n/// O.\nlet o: Out = { a: { hp: 2 } }\n"

// poisonCase is an edit leaving its root without a value, and the layers it is applied under.
type poisonCase struct {
	name   string
	src    string
	op     edit.Operation
	layers []string
}

var poisonCases = []poisonCase{
	{"AddEntry missing hp, hm active", poisonSrc, edit.Operation{Kind: edit.OpAddEntry, Path: "d:ms", Key: edit.Key("bat"), Value: edit.Obj{}}, []string{"hm"}},
	{"AddEntry missing hp, hm declared", poisonSrc, edit.Operation{Kind: edit.OpAddEntry, Path: "d:ms", Key: edit.Key("bat"), Value: edit.Obj{}}, nil},
	{"Set of a record missing hp, no layer", recordPoisonSrc, edit.Operation{Kind: edit.OpSet, Path: "d:o.a", Value: edit.Obj{}}, nil},
}

// poisonFS holds src, and with poisonSrc the hm layer amending ms.
func poisonFS(src string) mapFS {
	fs := mapFS{"law/project.canon": file(projectCanon), caseLineFile: file(src)}
	if src == poisonSrc {
		fs["law/d/hm.layer.canon"] = file("package d\nlayer hm\n\namend ms {\n  wolf.hp: 5\n}\n")
	}
	return fs
}

// DECISIONS 466, 273, API.md E22, E17a: with AllowErrors, an edit leaving its root without a value
// whose Undo no operation applies (ErrNoValue) is refused so: by Apply's replay beside an active
// layer's line, else by CheckUndo on the re-check's analysis of its result, naming op 0.
func TestPoisonUndoRefused(t *testing.T) {
	ctx := context.Background()
	for _, c := range poisonCases {
		s := open(t, poisonFS(c.src), c.layers, "", "d")
		plan, err := edit.Apply(ctx, s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}, AllowErrors: true})
		if err == nil {
			after := open(t, written(poisonFS(c.src), plan), c.layers, "", "d")
			err = edit.CheckUndo(ctx, after.env, after.snap, plan)
		}
		var oe *edit.OpError
		if !errors.Is(err, edit.ErrNoValue) || !errors.As(err, &oe) || oe.Index != 0 {
			t.Errorf("%s with AllowErrors: %v; want ErrNoValue at operation 0", c.name, err)
		}
	}
}

// DECISIONS 466, API.md E22, E17a: CheckUndo passes an edit whose result gives every root of its
// Undo a value, analysing nothing more.
func TestCheckUndoClean(t *testing.T) {
	ctx := context.Background()
	op := edit.Operation{Kind: edit.OpAddEntry, Path: "d:ms", Key: edit.Key("bat"), Value: edit.Obj{"hp": edit.Int(2)}}
	s := open(t, poisonFS(poisonSrc), nil, "", "d")
	plan, err := edit.Apply(ctx, s.env, s.snap, edit.Request{Ops: []edit.Operation{op}, AllowErrors: true})
	if err != nil {
		t.Fatal(err)
	}
	after := open(t, written(poisonFS(poisonSrc), plan), nil, "", "d")
	if err := edit.CheckUndo(ctx, after.env, after.snap, plan); err != nil {
		t.Errorf("AddEntry ms.bat { hp: 2 }: CheckUndo %v, want nil", err)
	}
}

// DECISIONS 466, API.md E19, E22: without AllowErrors the same edits are the re-check's: Apply plans
// them, never refused for their Undo (an Evaluate draft so evaluates, V13).
func TestPoisonUndoRechecked(t *testing.T) {
	for _, c := range poisonCases {
		s := open(t, poisonFS(c.src), c.layers, "", "d")
		if plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}}); err != nil || plan == nil {
			t.Errorf("%s: %v, want it planned for the re-check", c.name, err)
		}
	}
}
