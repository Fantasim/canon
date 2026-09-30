package canon

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
)

// The budget sweep's project: a spends most of the budget on a value no one reads, and on one
// sized by a JSON file of b's it loads too; b holds a value whose cost x sets, a dependent field
// and a failing check; c imports b; d spends as much as a.
const (
	sweepRoot    = "/law"
	sweepProject = "project acme {\n  canon: \"0.1\"\n  budget: %d\n}\n"
	sweepA       = `/// A.
package a

/// Heavy.
let heavy: Int = [i * 2 for i in 0..20].len()

/// B's numbers, read by a too.
let ys: [Int] = load("../b/b.json")

/// As costly as ys says.
let echo: Int = [i for i in 0..ys[0]].len()
`
	sweepB = `/// B.
package b

/// A goal.
enum Goal { kill, collect }

/// What a goal aims at.
type Target(g: Goal) = match g {
  kill => String
  collect => Int
}

/// An item.
record Item {
  /// How many.
  count: Int
  /// Its goal.
  goal: Goal
  /// What it aims at.
  target: Target(goal)

  check count > 0 else "count must be positive"
}

/// X.
let x: Int = 1

/// As costly as x says.
let y: Int = [i for i in 0..x].len()

/// B's numbers.
let xs: [Int] = load("b.json")

/// The items.
let items: table Item = {
  sword { count: 0, goal: kill, target: "wolf" }
}
`
	sweepC     = "/// C.\npackage c\n\nimport b\n\n/// Y.\nlet y: Int = b.x + 1\n"
	sweepD     = "/// D.\npackage d\n\n/// Heavy, spent after b.\nlet heavy: Int = [i * 2 for i in 0..20].len()\n"
	sweepJSON0 = "[1]\n"
	sweepPkg   = "b"  // the package the edits change
	sweepCap   = 5000 // a budget past the cold need: the sweep fails rather than loop on
	sweepTwice = 2    // the sweep goes on to this many times the cold need
)

// sweepEdits are the two edits of each scenario (the first also creates the journal's
// directory): b's cost raised; b's cost lowered from past the budget, which the analysis before
// then spent before reaching d, an untouched package after b; a file of b's that a reads changed.
var sweepEdits = []struct {
	path   string
	values []int64
}{{"b:x", []int64{2, 20}}, {"b:x", []int64{80, 2}}, {"b:xs[0]", []int64{2, 30}}}

// sweepPaths are the values the sweep evaluates, the unqualified roots among them.
var sweepPaths = []string{"b:x", "x", "b:y", "b:xs", "xs", "b:items", "b:items.sword", "b:items.sword.target", "c:y", "a:echo"}

// API.md V13, V14, E17, DECISIONS 244 (log-2026-09-29 M4 P14-r, P14-r2): at every budget from 1
// to twice the cold need, after edits, Evaluate gives what the snapshot's every-package analysis
// gives, the one Value reads, with or without a draft, qualified or not, whichever it reads.
func TestEvaluateBudgetSweep(t *testing.T) {
	for i := range sweepEdits {
		need, covered, applied := 0, 0, 0
		for budget := 1; need == 0 || budget <= sweepTwice*need; budget++ {
			if budget > sweepCap {
				t.Fatalf("no budget up to %d holds the whole project", sweepCap)
			}
			held, cov, edited := sweepAt(t, budget, i)
			if held && need == 0 {
				need = budget
			}
			if cov {
				covered++
			}
			if edited {
				applied++
			}
		}
		if applied == 0 || sweepEdits[i].path == sweepEdits[0].path && covered == 0 {
			t.Errorf("scenario %d: edits applied at %d budgets, the touched analysis read at %d", i, applied, covered)
		}
	}
}

// sweepAt compares, at budget after scenario i's edits, every path's evaluations with the
// every-package analysis's: whether that analysis spent less than the budget, and whether the
// edit kept b's touched one.
func sweepAt(t *testing.T, budget, i int) (held, covered, edited bool) {
	t.Helper()
	ctx := context.Background()
	p := sweepOpen(t, budget)
	sc := sweepEdits[i]
	for _, n := range sc.values {
		res, err := p.Edit(ctx, Edit{Ops: []Op{Set(sc.path, Int(n))}, AllowErrors: true, Normalize: true})
		edited = err == nil && res.Applied
	}
	s, err := p.read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	all, err := analyze(ctx, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	touched, err := workspace.Touching(ctx, s, sweepPkg)
	if err != nil {
		t.Fatal(err)
	}
	covered = workspace.Covered(s, touched)
	c := sweepCase{s: s, all: all, budget: budget}
	if _, err := p.Value(ctx, sc.path); edited && err == nil { // a draft resolves its op as Value does (E1)
		c.draft = []Op{Set(sc.path, Int(sc.values[len(sc.values)-1]))}
	}
	for _, path := range sweepPaths {
		sweepPath(t, p, c, path)
	}
	for _, f := range all.Result().List {
		if f.Code == diag.E4401.Def().Code {
			return false, covered, edited
		}
	}
	return true, covered, edited
}

// sweepCase is one budget's snapshot, its every-package analysis, and the draft that remakes the
// last edit, none when the edits were not applied.
type sweepCase struct {
	s      *workspace.Snapshot
	all    *build.Analysis
	budget int
	draft  []Op
}

// sweepPath compares path's Evaluate, with and without a draft, and the value it reads, with the
// every-package analysis's.
func sweepPath(t *testing.T, p *Project, c sweepCase, path string) {
	t.Helper()
	want := sweepEval(t, p, evalOn{s: c.s, a: c.all, lang: p.lang}, path)
	if got := sweepPublic(t, p, EvalRequest{Path: path}); got != want {
		t.Errorf("budget %d, %s: Evaluate\n%s\nevery package\n%s", c.budget, path, got, want)
	}
	if got, all := sweepValue(t, p, c.s, path, false), sweepValue(t, p, c.s, path, true); got != all {
		t.Errorf("budget %d, %s: the value Evaluate reads\n%s\nValue's\n%s", c.budget, path, got, all)
	}
	if c.draft == nil {
		return
	}
	if got := sweepPublic(t, p, EvalRequest{Path: path, Draft: c.draft}); got != want {
		t.Errorf("budget %d, %s: with a draft\n%s\nwithout\n%s", c.budget, path, got, want)
	}
}

// sweepOpen is the sweep's project at budget, in memory.
func sweepOpen(t *testing.T, budget int) *Project {
	t.Helper()
	fsys := newWriteFS(map[string][]byte{
		sweepRoot + "/project.canon": fmt.Appendf(nil, sweepProject, budget),
		sweepRoot + "/a/a.canon":     []byte(sweepA), sweepRoot + "/b/b.canon": []byte(sweepB), sweepRoot + "/c/c.canon": []byte(sweepC),
		sweepRoot + "/b/b.json": []byte(sweepJSON0), sweepRoot + "/d/d.canon": []byte(sweepD),
	}, nil)
	p, err := Open(sweepRoot, Options{FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// sweepPublic is Evaluate's result as JSON, its revision and dropped values left out, or its error.
func sweepPublic(t *testing.T, p *Project, r EvalRequest) string {
	t.Helper()
	res, err := p.Evaluate(context.Background(), r)
	if err != nil {
		return "error: " + err.Error()
	}
	res.Revision, res.Dropped = "", nil
	return sweepJSON(t, res)
}

// sweepEval is evaluateOn's result on on as JSON, or its error.
func sweepEval(t *testing.T, p *Project, on evalOn, path string) string {
	t.Helper()
	res, err := p.evaluateOn(context.Background(), on, path)
	if err != nil {
		return "error: " + err.Error()
	}
	res.Dropped = nil
	return sweepJSON(t, res)
}

func sweepJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// sweepValue is path as a Value reads it, its text, type with the arguments bound, origin
// (provenance) and editability: from the analysis Evaluate reads, or with all every package's.
func sweepValue(t *testing.T, p *Project, s *workspace.Snapshot, path string, all bool) string {
	t.Helper()
	ctx := context.Background()
	parsed, err := edit.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := evalAnalysis(ctx, s, parsed.Package, false)
	if all {
		a, err = analyze(ctx, s, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	snap := &snapshot{a: a, s: edit.NewSnapshot(a), editLayer: p.editLayer, layers: p.layers, types: newTypeEncoder(ctx, a), ctx: ctx}
	v, err := snap.resolve(path, parsed)
	if err != nil {
		return "error: " + err.Error()
	}
	return sweepJSON(t, []any{v.Path, v.Text, v.Kind, v.Type, v.Origin, v.Editable})
}
