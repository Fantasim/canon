package eval_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// worldHost is the harness host plus what a world needs: loads and verifications through its evaluator.
type worldHost struct {
	*host
	checked *check.Program
}

func (h worldHost) LoadInto(_ context.Context, ev *eval.Evaluator, e *syntax.LoadExpr, t types.Type, _ check.Bags) (value.Value, bool) {
	if h.load == nil {
		return nil, false
	}
	return h.load(ev, e, t)
}

func (h worldHost) VerifyInto(ctx context.Context, ev *eval.Evaluator, root eval.Root, v value.Value, bags check.Bags) bool {
	res, err := verify.New(ev, h.checked, bags, nil).Check(ctx, root, v)
	return settled(ev, root, res, err)
}

func (h worldHost) VerifyLiteralsInto(ctx context.Context, ev *eval.Evaluator, root eval.Root, lits []eval.KeptLiteral, bags check.Bags) {
	vr := verify.New(ev, h.checked, bags, nil)
	for _, l := range lits {
		res, err := vr.CheckLiteral(ctx, root, l.Value, l.Type)
		settled(ev, root, res, err)
	}
}

// settled applies a verification's result through ev, as build's host does.
func settled(ev *eval.Evaluator, root eval.Root, res verify.Result, err error) bool {
	if err != nil {
		return false
	}
	if res.Poisoned {
		ev.Poison(root)
	}
	for _, u := range res.Unbound {
		ev.ReportUnbound(root, u.Ref, u.Path)
	}
	return res.Valid
}

// packages builds value subjects as subjects does, and package subjects as build will: stages A
// to D of the world, every bag mapped to the capture.
type packages struct {
	subjects
	worlds *[]worldSeen
}

// worldSeen is what one world settled: each root of its order and its value, nil when poisoned.
type worldSeen struct {
	roots  []eval.Root
	values []value.Value
	ev     *eval.Evaluator
}

// the harness builds package subjects as build's subjects will
var _ eval.PackageBuilder = packages{}

// Build builds a value subject; a package subject is never one, so there is none to build.
func (p packages) Build(ctx context.Context, v value.Value, capture *diag.Bag) {
	if v != nil {
		p.subjects.Build(ctx, v, capture)
	}
}

func (p packages) BuildPackage(ctx context.Context, w *eval.World, capture *diag.Bag) {
	ev, order := w.Evaluator(), w.Order()
	bags := check.Bags{}
	for _, cp := range p.b.checked.Packages {
		bags[cp.Path] = capture
	}
	for _, root := range order {
		ev.Force(ctx, root)
	}
	ev.BeginVerification(ctx)
	runner := rules.New(checks{ev}, p.b.checked, bags)
	seen := worldSeen{ev: ev}
	for _, root := range order {
		v, ok := ev.Force(ctx, root)
		if ok {
			_ = runner.Instances(ctx, root, v)
		}
		seen.roots, seen.values = append(seen.roots, root), append(seen.values, v)
	}
	for _, cp := range p.b.checked.Packages {
		if cp.Path == p.pkg {
			_ = runner.Package(ctx, cp)
		}
	}
	*p.worlds = append(*p.worlds, seen)
}

// testRun is `canon test` over a checked program: a fresh evaluator whose values tests force,
// verified and shared, on per-package budgets.
type testRun struct {
	b      *build
	ev     *eval.Evaluator
	bags   check.Bags
	worlds []worldSeen
}

// newTestRun is a test run over b's program, its host serving worlds.
func newTestRun(b *build, opt eval.Options) *testRun {
	tr := &testRun{b: b, bags: check.Bags{}}
	for _, cp := range b.checked.Packages {
		tr.bags[cp.Path] = diag.NewBag(b.prog.fs, cp.Path)
	}
	h := &host{}
	tr.ev = eval.New(b.checked, worldHost{host: h, checked: b.checked}, tr.bags, opt)
	h.ev, h.verifier = tr.ev, verify.New(tr.ev, b.checked, tr.bags, nil)
	tr.ev.BeginVerification(context.Background())
	return tr
}

// builder is the Builder of pkg's tests, which builds package subjects.
func (tr *testRun) builder(pkg string) packages {
	return packages{subjects: subjects{b: &build{prog: tr.b.prog, checked: tr.b.checked, ev: tr.ev}, pkg: pkg}, worlds: &tr.worlds}
}

// run runs pkg's test named name, nil when there is none.
func (tr *testRun) run(t *testing.T, pkg, name string) *eval.TestRun {
	t.Helper()
	for _, cp := range tr.b.selected([]string{pkg}) {
		for _, obj := range cp.Decls {
			if d, ok := obj.Decl().(*syntax.TestDecl); ok && obj.Kind() == check.ObjTest && eval.TestName(d) == name {
				res := tr.ev.Test(context.Background(), d, tr.builder(pkg))
				return &res
			}
		}
	}
	t.Fatalf("no test %q in %s", name, pkg)
	return nil
}

// all runs every test of every package, in order, and prints each outcome, the expects with
// their captured findings and the worlds they built.
func (tr *testRun) all() string {
	var sb strings.Builder
	for _, cp := range tr.b.checked.Packages {
		for _, obj := range cp.Decls {
			d, ok := obj.Decl().(*syntax.TestDecl)
			if !ok || obj.Kind() != check.ObjTest {
				continue
			}
			from := len(tr.worlds)
			res := tr.ev.Test(context.Background(), d, tr.builder(cp.Path))
			fmt.Fprintf(&sb, "%s %s: failed %t, stopped %t, broken %t", cp.Path, testTitle(obj.File(), d), res.Failed, res.Stopped, res.Broken)
			for _, f := range res.Errors {
				fmt.Fprintf(&sb, ", stopped by %s at line %d (%s)", f.Code, spanLine(obj.File(), f.Span), strings.ReplaceAll(f.Message, "\n", "; "))
			}
			sb.WriteString("\n")
			printWorldExpects(&sb, obj.File(), res.Expects)
			printWorlds(&sb, tr.worlds[from:])
		}
	}
	return sb.String()
}

// printWorldExpects prints each expect, its captured findings with their paths and its poisoned read.
func printWorldExpects(sb *strings.Builder, f *syntax.File, xs []eval.Expect) {
	for _, x := range xs {
		fmt.Fprintf(sb, "  expect line %d: passed %t", line(f, x.Stmt), x.Passed)
		for _, c := range x.Captured {
			fmt.Fprintf(sb, " [%s %s: %s]", c.Code, c.Path, c.Message)
		}
		if x.Poisoned != "" {
			fmt.Fprintf(sb, " (reads poisoned %s)", x.Poisoned)
		}
		sb.WriteString("\n")
	}
}

// printWorlds prints the values each world settled, in its order.
func printWorlds(sb *strings.Builder, ws []worldSeen) {
	for i, w := range ws {
		fmt.Fprintf(sb, "  world %d:", i+1)
		for j, root := range w.roots {
			text := "poisoned"
			if w.values[j] != nil {
				text = w.values[j].CanonText()
			}
			fmt.Fprintf(sb, " %s = %s;", root.Name, text)
		}
		sb.WriteString("\n")
	}
}

// findings renders the test run's own bags, which a world never reports into.
func (tr *testRun) findings(t *testing.T) string {
	t.Helper()
	return (&build{prog: tr.b.prog, bags: tr.bags}).findings(t)
}

// EVALUATION.md §10.5: the archive's first line names the items of §10.5 it proves.
func TestPackageSubjects(t *testing.T) {
	golden.Run(t, "testdata/worlds/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		opt := caseOptions(t, c)
		opt.Layers = strings.Fields(string(archiveFile(c.Archive, layersFile)))
		b := runBuild(t, fromArchive(t, c.Archive), eval.Options{Layers: opt.Layers})
		tr := newTestRun(b, opt)
		out := tr.all()
		if err := tr.ev.Err(); err != nil {
			t.Errorf("internal error: %v", err)
		}
		return []byte(out + "\n" + tr.findings(t))
	}, golden.Expected(outcomesFile))
}
