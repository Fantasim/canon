package build

import (
	"context"
	"regexp"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// TestResult is one canon test: the selected tests in order, with their failures (API.md §13.2).
type TestResult struct {
	Files    diag.Files // resolves the findings' spans
	Static   Result     // phases 1-2's error findings of the loaded packages and project.canon (ADR-0004)
	Tests    []TestCase
	Revision string
}

// TestCase is one test block's outcome (API.md B3).
type TestCase struct {
	Package, Name string
	Loc           source.Location // from the test keyword to the end of the block
	Passed        bool
	Failures      []ExpectFailure
}

// ExpectFailure is a failing expect, or with Expect "" what stopped or broke the test: one finding, or a poisoned read (CLI.md §3.5).
type ExpectFailure struct {
	Loc      source.Location // the expect statement, the stopping error, or the poisoned read
	Expect   string
	Outcome  string // passes, fails or warns; "" for `expect c` and a stop
	Op       string // a comparison's operator, when both its operands were evaluated
	Expected string // that comparison's right operand, as text
	Got      string // its left operand, or the value of any other Boolean, as text
	Poisoned string // the poisoned top-level value read
	Findings []diag.Finding
	Cause    []diag.Finding // the errors that poisoned it
}

// Test runs phases 1 and 2, then the selected tests match accepts (nil: all) in order, sharing one evaluator's budget and values (EVALUATION.md §1, §10.1).
func (p *Project) Test(ctx context.Context, selectors []string, match *regexp.Regexp) (*TestResult, error) {
	r, err := p.prepare(ctx, selectors)
	if err != nil {
		return nil, err
	}
	if err := r.check(ctx); err != nil {
		return nil, err
	}
	res := &TestResult{Files: r.s.set, Static: r.staticErrors(), Revision: r.s.revision()}
	res.Static.Revision = res.Revision
	r.newHost(r.throwawayBags())
	r.host.logBags, r.host.causes = r.throwawayBags, map[eval.Root][]diag.Finding{}
	r.ev.BeginVerification(ctx)
	for _, cp := range r.prog.Packages {
		if r.selects(cp.Path) {
			res.Tests = append(res.Tests, r.runTests(ctx, cp, match, res.Static.List)...)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.host.failure(r.s.set, r.prog); err != nil {
		return nil, err
	}
	return res, nil
}

// staticErrors is every error finding phases 1-2 left in project.canon and the loaded packages, the selected ones counted (ADR-0004).
func (r *run) staticErrors() Result {
	out := Result{Findings: Findings{Files: r.s.set}}
	out.addErrors(r.s.own)
	for _, u := range r.loaded {
		out.addErrors(r.s.bag(u.Name))
	}
	for _, u := range r.selected {
		out.Packages = append(out.Packages, u.Name)
	}
	out.Summary.Packages = len(r.selected)
	return out
}

// runTests runs the tests of cp that match accepts, in declaration order.
func (r *run) runTests(ctx context.Context, cp *check.Package, match *regexp.Regexp, static []diag.Finding) []TestCase {
	var out []TestCase
	b := subjects{pkg: cp.Path, ev: r.ev, r: r, host: r.host}
	for _, obj := range cp.Decls {
		d, ok := obj.Decl().(*syntax.TestDecl)
		if !ok || obj.Kind() != check.ObjTest || match != nil && !match.MatchString(eval.TestName(d)) {
			continue
		}
		run := r.ev.Test(ctx, d, b)
		out = append(out, r.testCase(cp.Path, obj.File(), d, run, static))
	}
	return out
}

// testCase is a test's outcome: its failing expects, then what stopped or broke it (EVALUATION.md §10.4).
func (r *run) testCase(pkg string, f *syntax.File, d *syntax.TestDecl, run eval.TestRun, static []diag.Finding) TestCase {
	tc := TestCase{Package: pkg, Name: eval.TestName(d), Loc: r.s.set.Locate(testSpan(f, d)), Passed: !run.Failed}
	for _, x := range run.Expects {
		if !x.Passed {
			tc.Failures = append(tc.Failures, r.expectFailure(f, x))
		}
	}
	tc.Failures = append(tc.Failures, r.stops(run, static, f.Span(d))...)
	return tc
}

// stops is the test's hard error, its poisoned read and cause (not under E4401), or its static errors (CLI.md §3.5).
func (r *run) stops(run eval.TestRun, static []diag.Finding, decl source.Span) []ExpectFailure {
	var out []ExpectFailure
	errs := run.Errors
	switch {
	case run.Broken:
		errs = brokenBy(static, decl)
	case run.Poisoned != "" && !slices.ContainsFunc(errs, exhausts):
		out = append(out, ExpectFailure{Loc: r.s.set.Locate(run.PoisonSpan), Poisoned: run.Poisoned, Cause: r.poisonCause(run.PoisonRoot)})
	}
	for _, f := range errs {
		out = append(out, ExpectFailure{Loc: r.s.set.Locate(f.Span), Findings: []diag.Finding{f}})
	}
	return out
}

// exhausts reports the budget's end, E4401 (EVALUATION.md §12.2).
func exhausts(f diag.Finding) bool {
	return f.Code == diag.E4401.Def().Code
}

// poisonCause is the evaluation or verification errors that first poisoned root's value (EVALUATION.md §7.2).
func (r *run) poisonCause(root eval.Root) []diag.Finding {
	c := r.ev.PoisonCause(root)
	return slices.Concat(c.Findings, r.host.causes[c.Root])
}

// brokenBy is the static errors inside a broken test's declaration, else every static error:
// one of them broke a declaration it names.
func brokenBy(static []diag.Finding, decl source.Span) []diag.Finding {
	var in []diag.Finding
	for _, f := range static {
		if f.Span.File == decl.File && f.Span.Start >= decl.Start && f.Span.Start < decl.End {
			in = append(in, f)
		}
	}
	if len(in) == 0 {
		return static
	}
	return in
}

// testSpan runs from a test's `test` keyword to the end of its block (CLI.md §3.5).
func testSpan(f *syntax.File, d *syntax.TestDecl) source.Span {
	sp := f.Span(d)
	if kw := d.Name.First() - 1; kw > syntax.NoTok && f.Tokens[kw].Kind == syntax.KwTest {
		sp.Start = f.Tokens[kw].Start
	}
	return sp
}

// expectFailure is a failed expect: its source text, expected and got, and captured findings.
func (r *run) expectFailure(f *syntax.File, x eval.Expect) ExpectFailure {
	sp := f.Span(x.Stmt)
	out := ExpectFailure{Loc: r.s.set.Locate(sp), Expect: string(f.Src.Content[sp.Start:sp.End]), Poisoned: x.Poisoned, Findings: x.Captured}
	if x.Stmt.Outcome != nil {
		out.Outcome = x.Stmt.Outcome.Name
	}
	switch {
	case x.Poisoned != "":
		out.Cause = r.poisonCause(x.PoisonRoot)
	case x.Compared: // known by its operator, never by its operands' text (a String may be empty)
		out.Op, out.Expected, out.Got = x.Op.String(), x.Right, x.Left
	default:
		out.Got = x.Value
	}
	return out
}
