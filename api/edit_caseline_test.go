package canon_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// caseLineSrc is a box a layer amends below each field: hb at sh.r, ho at osh.r, hl at ls[1]; and
// cp, spreading bb, whose sh.r hc amends.
const caseLineSrc = `package d

/// A shape.
variant Shape {
  /// A circle.
  circle {
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
}

/// The box.
let bb: Box = { sh: circle { r: 2 }, osh: circle { r: 3 }, ls: [1, 2] }

/// A copy.
let cp: Box = { ...bb }
`

const caseLineData = "/law/d/d.canon"

// caseLineOpen opens caseLineSrc with its layers, layers active, and its file system.
func caseLineOpen(t *testing.T, layers ...string) (*canon.Project, *memFS) {
	t.Helper()
	opts := project(map[string]string{"project.canon": editLaw["project.canon"], "d/d.canon": caseLineSrc,
		"d/hb.layer.canon": "package d\nlayer hb\n\namend bb {\n  sh.r: 9\n}\n",
		"d/ho.layer.canon": "package d\nlayer ho\n\namend bb {\n  osh.r: 8\n}\n",
		"d/hl.layer.canon": "package d\nlayer hl\n\namend bb {\n  ls[1]: 9\n}\n",
		"d/hc.layer.canon": "package d\nlayer hc\n\namend cp {\n  sh.r: 9\n}\n"})
	opts.Layers = layers
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, opts.FS.(*memFS)
}

// caseLineCases are base edits leaving the active layer's line below their path in error (E1905).
var caseLineCases = []struct {
	name, layer string
	op          canon.Op
}{
	{"SetCase to a case without the amended field", "hb", canon.SetCase("d:bb.sh", "square", canon.Obj{"side": canon.Int(4)})},
	{"SetCase of an optional variant", "ho", canon.SetCase("d:bb.osh", "square", nil)},
	{"Set of the variant to another case", "hb", canon.Set("d:bb.sh", canon.Source("square { side: 4 }"))},
	{"Set of an optional variant to none", "ho", canon.Set("d:bb.osh", canon.Source("none"))},
	{"Remove before the amended index", "hl", canon.Remove("d:bb.ls[0]")},
	{"Set of a list shorter than the amended index", "hl", canon.Set("d:bb.ls", canon.Source("[5]"))},
}

// API.md E19, E22 (DECISIONS 273, 466): without AllowErrors, the re-check refuses each such edit
// with the broken line's E1905, and nothing is written.
func TestEditCaseLineRejected(t *testing.T) {
	for _, c := range caseLineCases {
		p, fsys := caseLineOpen(t, c.layer)
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}})
		var re *canon.RejectedError
		got, _ := fsys.ReadFile(caseLineData)
		if !errors.As(err, &re) || res == nil || res.Applied || !slices.ContainsFunc(re.Findings, isCode(diag.E1905.Def().Code)) || string(got) != caseLineSrc {
			t.Errorf("%s under %s: %v, want it rejected with %s and d.canon untouched", c.name, c.layer, err, diag.E1905.Def().Code)
		}
	}
}

// API.md E22, E23, W10 (DECISIONS 273, 466): with AllowErrors, each such edit's Undo would be
// refused `layered`, so the edit is refused so, naming the layer, and nothing is written.
func TestEditCaseLineAllowErrorsLayered(t *testing.T) {
	for _, c := range caseLineCases {
		p, fsys := caseLineOpen(t, c.layer)
		_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}, AllowErrors: true})
		var ne *canon.NotEditableError
		got, _ := fsys.ReadFile(caseLineData)
		if !errors.As(err, &ne) || ne.Reason != canon.ReasonLayered || ne.Layer != c.layer || string(got) != caseLineSrc {
			t.Errorf("%s under %s with AllowErrors: %v, want NotEditable layered by %s, d.canon untouched", c.name, c.layer, err, c.layer)
		}
	}
}

// API.md E22 (DECISIONS 273): with AllowErrors, a SetCase of bb.sh leaving cp, which hc amends,
// uncomputed keeps its plain Undo, unverified: no layer line meets it, and it gives d.canon back.
func TestEditCaseLineUnverifiedKept(t *testing.T) {
	p, fsys := caseLineOpen(t, "hc")
	op := canon.SetCase("d:bb.sh", "square", canon.Obj{"side": canon.Int(4)})
	res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{op}})
	if err != nil || !res.Applied || !slices.ContainsFunc(res.Findings, isCode(diag.E1905.Def().Code)) {
		t.Fatalf("SetCase bb.sh under hc with AllowErrors: %+v, %v, want it applied with hc's %s", res, err, diag.E1905.Def().Code)
	}
	if _, err := p.Edit(context.Background(), canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("its Undo %+v: %v, want it applied", res.Undo, err)
	}
	if got, _ := fsys.ReadFile(caseLineData); string(got) != caseLineSrc {
		t.Errorf("its Undo leaves d.canon:\n%s", got)
	}
}

// poisonLaw is poisonedLaw's plain table, which hm amends, and a let n beside it.
func poisonLaw() map[string]string {
	law := poisonedLaw(false)
	law["project.canon"] = editLaw["project.canon"]
	law["p/hm.layer.canon"] = "package p\nlayer hm\n\namend ms {\n  wolf.hp: 5\n}\n"
	law["p/q.canon"] = "package p\n\nlet n: Int = 1\n"
	return law
}

// poisonOpen opens law with layers active, and its file system.
func poisonOpen(t *testing.T, law map[string]string, layers []string) (*canon.Project, *memFS) {
	t.Helper()
	opts := project(law)
	opts.Layers = layers
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, opts.FS.(*memFS)
}

var poisonBat = canon.AddEntry("p:ms", canon.Key("bat"), canon.Obj{})

// API.md E22, E19 (DECISIONS 273, 466): an AllowErrors AddEntry missing hp in a table a layer amends
// leaves it without a value, and its Undo's Remove gets ErrNoValue: refused so, hm active or not.
func TestEditPoisonAmendedRefused(t *testing.T) {
	law := poisonLaw()
	for _, layers := range [][]string{{"hm"}, nil} {
		p, fsys := poisonOpen(t, law, layers)
		res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{poisonBat}})
		got, _ := fsys.ReadFile("/law/p/p.canon")
		if !errors.Is(err, canon.ErrNoValue) || res != nil && res.Applied || string(got) != law["p/p.canon"] {
			t.Errorf("AddEntry p:ms.bat {} with AllowErrors, layers %v: %v, want ErrNoValue, p.canon untouched", layers, err)
		}
	}
}

// API.md E19, E22 (DECISIONS 273, 466): without AllowErrors the same AddEntry is the re-check's: it
// is rejected with its findings, not refused for its Undo, and nothing is written.
func TestEditPoisonAmendedRechecked(t *testing.T) {
	law := poisonLaw()
	for _, layers := range [][]string{{"hm"}, nil} {
		p, fsys := poisonOpen(t, law, layers)
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{poisonBat}})
		var re *canon.RejectedError
		got, _ := fsys.ReadFile("/law/p/p.canon")
		if !errors.As(err, &re) || res == nil || res.Applied || string(got) != law["p/p.canon"] {
			t.Errorf("AddEntry p:ms.bat {}, layers %v: %v, want it rejected by the re-check, p.canon untouched", layers, err)
		}
	}
}

// API.md V13, E22 (DECISIONS 466): a draft is never written and has no Undo, so the same AddEntry as
// a draft evaluates, hm active or not: p:n is read, and p:ms has no value of its own (no op refused).
func TestEditPoisonDraftEvaluates(t *testing.T) {
	law := poisonLaw()
	for _, layers := range [][]string{{"hm"}, nil} {
		p, _ := poisonOpen(t, law, layers)
		if _, err := p.Evaluate(context.Background(), canon.EvalRequest{Path: "p:n", Draft: []canon.Op{poisonBat}}); err != nil {
			t.Errorf("Evaluate p:n with the draft AddEntry p:ms.bat {}, layers %v: %v, want it evaluated", layers, err)
		}
		_, err := p.Evaluate(context.Background(), canon.EvalRequest{Path: "p:ms", Draft: []canon.Op{poisonBat}})
		var pe *canon.PathError
		if !errors.As(err, &pe) || !errors.Is(err, canon.ErrNoValue) || pe.Op != -1 {
			t.Errorf("Evaluate p:ms with the draft AddEntry p:ms.bat {}, layers %v: %v, want p:ms without a value, no op refused", layers, err)
		}
	}
}

// API.md E22, E23 (DECISIONS 273, 466): with AllowErrors and no layer, a Set leaving a plain record
// let without a value has an Undo no operation applies (ErrNoValue): the edit is refused so.
func TestEditRecordPoisonRefused(t *testing.T) {
	src := "package r\n\nrecord In {\n  hp: Int\n}\n\nrecord Out {\n  a: In = { hp: 1 }\n  b: Int = 1\n}\n\nlet o: Out = { a: { hp: 2 } }\n"
	law := map[string]string{"project.canon": editLaw["project.canon"], "r/r.canon": src}
	p, fsys := poisonOpen(t, law, nil)
	res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{canon.Set("r:o.a", canon.Obj{})}})
	got, _ := fsys.ReadFile("/law/r/r.canon")
	if !errors.Is(err, canon.ErrNoValue) || res != nil && res.Applied || string(got) != src {
		t.Errorf("Set r:o.a {} with AllowErrors: %v, want ErrNoValue, r.canon untouched", err)
	}
}

// API.md W5, X1, E22: a refused inverse is named by the operation it undoes, not the first or last.
func TestEditUndoRefusalOpIndex(t *testing.T) {
	p, _ := poisonOpen(t, poisonLaw(), nil)
	_, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{canon.Set("p:n", canon.Int(2)), poisonBat, canon.Set("p:n", canon.Int(3))}})
	var pe *canon.PathError
	if !errors.As(err, &pe) || !errors.Is(err, canon.ErrNoValue) || pe.Op != 1 || pe.Path != "p:ms" {
		t.Errorf("[Set p:n, AddEntry p:ms.bat {}, Set p:n] with AllowErrors: %v, want ErrNoValue at op 1 (p:ms)", err)
	}
	q, _ := caseLineOpen(t, "hb")
	_, err = q.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{canon.Set("d:bb.ls", canon.Source("[7, 8]")), caseLineCases[0].op}})
	var ne *canon.NotEditableError
	if !errors.As(err, &ne) || ne.Op != 1 || ne.Reason != canon.ReasonLayered || ne.Layer != "hb" {
		t.Errorf("[Set d:bb.ls, SetCase d:bb.sh square] under hb with AllowErrors: %v, want layered by hb at op 1", err)
	}
}
