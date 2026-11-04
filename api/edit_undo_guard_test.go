package canon_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// guardSrc is a loaded thing whose bits list, pairs list and plain list a dev and a qa layer amend.
const guardSrc = `/// G.
package d

/// A mark.
enum Mark @codes(UInt8) { a = 1, b = 2, c = 4 }

/// A stat.
record Stat {
  /// Which.
  attr: String
  /// How much.
  val: Int = 0
}

/// A thing.
record Thing {
  /// Marks.
  marks: [Mark] = [] @json(bits)
  /// Stats.
  stats: [Stat](..=3) = [] @json(pairs: ["k{i}", "v{i}"])
  /// Tags.
  tags: [String] = []
}

/// Loaded.
let t: Thing = load("t.json")
`

const (
	guardJSON = "{\n  \"marks\": 3,\n  \"k0\": \"x\",\n  \"v0\": 1,\n  \"k1\": \"y\",\n  \"v1\": 2,\n  \"tags\": [\n    \"p\",\n    \"q\"\n  ]\n}\n"
	guardDev  = "package d\nlayer dev\n\namend t {\n  stats[0].val: 50\n  marks[0]: c\n  tags[0]: \"DEV\"\n}\n"
	guardQA   = "package d\nlayer qa\n\namend t {\n  stats[1].val: 70\n}\n"
	guardData = "/law/d/t.json"
)

// guardOpen opens the guard project with layers active, and its file system.
func guardOpen(t *testing.T, layers ...string) (*canon.Project, *memFS) {
	t.Helper()
	return guardOpenFiles(t, map[string]string{"d/dev.layer.canon": guardDev, "d/qa.layer.canon": guardQA}, layers)
}

// guardOpenFiles opens the guard project with more files, layers active.
func guardOpenFiles(t *testing.T, more map[string]string, layers []string) (*canon.Project, *memFS) {
	t.Helper()
	files := map[string]string{"project.canon": editLaw["project.canon"], "d/d.canon": guardSrc, "d/t.json": guardJSON}
	for name, text := range more { //canon:unordered each file is stored under its own name
		files[name] = text
	}
	opts := project(files)
	opts.Layers = layers
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, opts.FS.(*memFS)
}

// API.md E22, E19 (DECISIONS 273, 463): a base edit that leaves an active layer amending an element
// it removed cannot be undone, its root no longer computed: the re-check refuses it with that
// layer's E1905 before anything is written.
func TestEditUndoGuardBrokenLayer(t *testing.T) {
	for _, c := range []struct {
		name   string
		layers []string
		op     canon.Op
	}{
		{"an emptied pairs list dev amends", []string{"dev"}, canon.Set("d:t.stats", canon.Source("[]"))},
		{"a removed element qa amends", []string{"dev", "qa"}, canon.Remove("d:t.stats[1]")},
	} {
		p, fsys := guardOpen(t, c.layers...)
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}})
		var re *canon.RejectedError
		got, _ := fsys.ReadFile(guardData)
		if !errors.As(err, &re) || res.Applied || !slices.ContainsFunc(re.Findings, isCode(diag.E1905.Def().Code)) || string(got) != guardJSON {
			t.Errorf("%s: %v, want it rejected with %s and t.json untouched", c.name, err, diag.E1905.Def().Code)
		}
	}
}

// API.md E22, W10 (EDIT-MATERIALIZE ruling): with dev active, a base Move in a list dev amends by
// position is refused `layered`, naming dev, since the Move back would meet dev's amendment.
func TestEditUndoGuardMove(t *testing.T) {
	p, fsys := guardOpen(t, "dev")
	_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Move("d:t.tags[1]", 0)}})
	var ne *canon.NotEditableError
	got, _ := fsys.ReadFile(guardData)
	if !errors.As(err, &ne) || ne.Reason != canon.ReasonLayered || ne.Layer != "dev" || string(got) != guardJSON {
		t.Errorf("Move under dev: %v, want NotEditable layered by dev, t.json untouched", err)
	}
}

// API.md V1, E18, E19 (EDIT-MATERIALIZE ruling): a base edit's bitmask check reads the base list
// [a, b], so a Set of marks[1] or an Add of c passes it; the re-check then refuses either, dev's
// marks[0]: c holding c twice in its view (E8102), and nothing is written; with no layer, it applies.
func TestEditBaseBitsJudgedInBase(t *testing.T) {
	for _, op := range []canon.Op{canon.Set("d:t.marks[1]", canon.Member("c")), canon.Add("d:t.marks", canon.Member("c"))} {
		p, fsys := guardOpen(t, "dev")
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{op}})
		var re *canon.RejectedError
		got, _ := fsys.ReadFile(guardData)
		if !errors.As(err, &re) || res.Applied || !slices.ContainsFunc(re.Findings, isCode(diag.E8102.Def().Code)) || string(got) != guardJSON {
			t.Errorf("%s %s under dev: %v, want it rejected by the re-check with %s, t.json untouched", op.Kind, op.Path, err, diag.E8102.Def().Code)
		}
		p0, f0 := guardOpenFiles(t, nil, nil)
		if _, err := p0.Edit(context.Background(), canon.Edit{Ops: []canon.Op{op}}); err != nil {
			t.Errorf("%s %s with no layer: %v, want it applied", op.Kind, op.Path, err)
		}
		if got, _ := f0.ReadFile(guardData); string(got) == guardJSON {
			t.Errorf("%s %s with no layer: t.json unchanged", op.Kind, op.Path)
		}
	}
}
