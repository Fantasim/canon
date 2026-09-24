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
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// subjects builds expect subjects with verify and rules, their findings captured (EVALUATION.md §10.2).
type subjects struct {
	b   *build
	pkg string
}

func (s subjects) Build(ctx context.Context, v value.Value, capture *diag.Bag) {
	bags := map[string]*diag.Bag{s.pkg: capture}
	root := eval.Root{Pkg: s.pkg}
	if _, err := verify.New(s.b.ev, s.b.checked, bags, nil).Check(ctx, root, v); err != nil {
		return
	}
	_ = rules.New(checks{s.b.ev}, s.b.checked, bags).Instances(ctx, root, v)
}

// runTests runs the tests of the selected packages in order (EVALUATION.md §10.1).
func runTests(t *testing.T, b *build, pkgs ...string) string {
	t.Helper()
	var sb strings.Builder
	for _, pkg := range b.selected(pkgs) {
		for _, obj := range pkg.Decls {
			d, ok := obj.Decl().(*syntax.TestDecl)
			if !ok || obj.Kind() != check.ObjTest {
				continue
			}
			res := b.ev.Test(context.Background(), d, subjects{b: b, pkg: pkg.Path})
			fmt.Fprintf(&sb, "%s %s: failed %t, stopped %t, broken %t", pkg.Path, testTitle(obj.File(), d), res.Failed, res.Stopped, res.Broken)
			if res.Poisoned != "" {
				fmt.Fprintf(&sb, ", reads poisoned %s", res.Poisoned)
			}
			sb.WriteString("\n")
			printExpects(&sb, obj.File(), res.Expects)
		}
	}
	return sb.String()
}

func printExpects(sb *strings.Builder, f *syntax.File, xs []eval.Expect) {
	for _, x := range xs {
		fmt.Fprintf(sb, "  expect line %d: passed %t", line(f, x.Stmt), x.Passed)
		for _, c := range x.Captured {
			fmt.Fprintf(sb, " [%s %s]", c.Code, c.Message)
		}
		if x.Poisoned != "" {
			fmt.Fprintf(sb, " (reads poisoned %s)", x.Poisoned)
		}
		if x.Left != "" || x.Right != "" {
			fmt.Fprintf(sb, " (left %s, right %s)", x.Left, x.Right)
		}
		sb.WriteString("\n")
	}
}

func testTitle(f *syntax.File, d *syntax.TestDecl) string {
	sp := f.Span(d.Name)
	return string(f.Src.Content[sp.Start:sp.End])
}

func line(f *syntax.File, n syntax.Node) int {
	l, _ := f.Src.Position(f.Span(n).Start)
	return l
}

// EVALUATION.md §10: the tests of sovcommon/time pass, `fails` matching a check's message.
func TestExampleTests(t *testing.T) {
	b := runBuild(t, fromExamples(t, "sovcommon/time"), eval.Options{}, "sovcommon.time")
	out := runTests(t, b, "sovcommon.time")
	if strings.Contains(out, "passed false") || strings.Contains(out, "failed true") {
		t.Errorf("tests:\n%s", out)
	}
	if strings.Count(out, "passed true") != 5 {
		t.Errorf("want 5 passing expects, got:\n%s", out)
	}
}

// EVALUATION.md §10.3, §10.4: expect forms, failures, a stopped test, a poisoned read.
func TestTests(t *testing.T) {
	golden.Run(t, "testdata/tests/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		b := runBuild(t, fromArchive(t, c.Archive), eval.Options{})
		return []byte(runTests(t, b) + "\n" + b.findings(t))
	}, golden.Expected(outcomesFile))
}

const outcomesFile = "outcomes.txt"
