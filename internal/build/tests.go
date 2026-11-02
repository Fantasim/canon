package build

import (
	"context"
	"math"
	"regexp"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/verify"
)

// TestResult is one canon test: the selected tests in order, with their failures (API.md §13.2).
type TestResult struct {
	Files    diag.Files // resolves the findings' spans
	Static   Result     // phases 1-2's errors of the loaded packages and project.canon, @codes, refused amendments, and the selected packages' warnings, W7101 included (API.md B5)
	Tests    []TestCase
	Revision string
	Manifest []byte // the build manifest of this test run (WIRE.md §10)
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

// Test runs phases 1 and 2, then the selected tests match accepts (nil: all) in order, sharing one evaluator's values and per-package budgets (EVALUATION.md §1, §10.1, §12.2).
func (p *Project) Test(ctx context.Context, selectors []string, match *regexp.Regexp) (*TestResult, error) {
	r, err := p.prepare(ctx, selectors)
	if err != nil {
		return nil, err
	}
	return r.test(ctx, match)
}

// test is Test over a prepared run, then canon check's run of its selection, whose warnings it reports (API.md B5).
func (r *run) test(ctx context.Context, match *regexp.Regexp) (*TestResult, error) {
	if err := interrupted(ctx, r.check(ctx)); err != nil {
		return nil, err
	}
	testBags := uncapped(r.throwawayBags()) // the test host's: their W7101 joins the static result (headerSkips)
	r.newHost(testBags)
	if err := r.testNames(); err != nil {
		return nil, err
	}
	static := r.staticErrors() // what can break a declaration: a @codes finding cannot (brokenBy)
	if err := r.verifyCodesBy(verify.NewShared(r.vix, r.ev, r.bags, r.assets)); err != nil {
		return nil, err
	}
	res := &TestResult{Files: r.s.set, Revision: r.s.revision()}
	r.host.logBags, r.host.causes = r.throwawayBags, map[eval.Root][]diag.Finding{}
	r.ev.BeginVerification(ctx)
	for _, cp := range r.prog.Packages {
		if r.selects(cp.Path) {
			res.Tests = append(res.Tests, r.runTests(ctx, cp, match, static.List)...)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.host.loader.FinishDefines() // once every test has forced its loads (WIRE.md §6.8)
	if err := r.host.failure(r.s.set, r.prog); err != nil {
		return nil, err
	}
	r.reportStableAmendments()
	r.reportRefused(r.refused)
	parts := r.errorParts() // taken before the check run adds its findings to the snapshot's bags
	warned, err := r.checkWarnings(ctx, r.headerSkips(testBags))
	if err != nil {
		return nil, err
	}
	res.Static = r.static(parts, warned)
	res.Static.Revision = res.Revision
	res.Manifest = r.manifest(r.inputs(), commandTest, nil)
	return res, nil
}

// testNames reports E5004 and E5005 of the selected packages, which are static (EVALUATION.md §10.1, §10.3).
func (r *run) testNames() error {
	runner := rules.NewShared(r.rix, checks{Evaluator: r.ev, failed: &r.failed}, r.bags)
	for _, cp := range r.prog.Packages {
		if r.selects(cp.Path) {
			if err := runner.Tests(cp); err != nil {
				return internal(err)
			}
		}
	}
	return nil
}

// staticErrors is every error finding phases 1-2 left in project.canon and the loaded packages, the selected ones counted (ADR-0004).
func (r *run) staticErrors() Result {
	return r.static(r.errorParts(), nil)
}

// errorParts is the error findings and counts of project.canon's bag, the selected orphans' and the loaded packages', as they are now (ADR-0004).
func (r *run) errorParts() map[*diag.Bag]Findings {
	bags := append([]*diag.Bag{r.s.own}, r.orphanBags()...)
	for _, u := range r.loaded {
		bags = append(bags, r.s.bag(u.Name))
	}
	out := make(map[*diag.Bag]Findings, len(bags))
	for _, b := range bags {
		part := Findings{Files: r.s.set}
		part.addErrors(b)
		out[b] = part
	}
	return out
}

// static is parts, each joined by the warnings warned holds for its bag's name, the selected packages counted (API.md B5).
func (r *run) static(parts map[*diag.Bag]Findings, warned map[string]Findings) Result {
	out := Result{Findings: Findings{Files: r.s.set}}
	out.join(warnedPart(parts[r.s.own], warned, "", diag.DefaultMaxFindings))
	for _, o := range r.orphans {
		out.join(warnedPart(parts[r.s.bag(o.Name)], warned, o.Name, r.p.findingLimit()))
	}
	for _, u := range r.loaded {
		out.join(warnedPart(parts[r.s.bag(u.Name)], warned, u.Name, r.p.findingLimit()))
	}
	for _, u := range r.selected {
		out.Packages = append(out.Packages, u.Name)
	}
	out.Summary.Packages = len(r.selected)
	return out
}

// warnedPart is part, the errors of the bag of name, joined by the warnings warned holds for name, kept under limit.
func warnedPart(part Findings, warned map[string]Findings, name string, limit int) Findings {
	if w, ok := warned[name]; ok {
		return withWarnings(part, w, limit)
	}
	return part
}

// join adds o's findings and counts.
func (f *Findings) join(o Findings) {
	f.List = append(f.List, o.List...)
	f.Summary = f.Summary.Merge(o.Summary)
}

// checkWarnings is, by selected package, the warnings canon check's run of r's selection reports
// on r's snapshot, with each W7101 of moved whose header that run reported none for (API.md B5).
func (r *run) checkWarnings(ctx context.Context, moved map[string][]diag.Finding) (map[string]Findings, error) {
	c := &run{p: r.p, s: r.s, selected: r.selected, orphans: r.orphans, loaded: r.loaded, bags: r.s.bagsOf(r.loaded), cached: true}
	a, err := c.analyzed(ctx) // Analyze's phases 2 to 7 (CLI.md §3.3)
	if err != nil {
		return nil, err
	}
	out := map[string]Findings{}
	for _, name := range r.warnedNames() {
		w := warningsOf(a.Result(), name)
		extra := c.unreported(name, moved[name])
		w.List = append(w.List, extra...)
		w.Summary.Warnings += len(extra)
		out[name] = w
	}
	return out, nil
}

// warnedNames is whose warnings canon check reports: project.canon ("") and the selected orphans and packages (API.md R2).
func (r *run) warnedNames() []string {
	names := []string{""}
	for _, o := range r.orphans {
		names = append(names, o.Name)
	}
	return append(names, r.selectedNames()...)
}

// warningsOf is the warnings res reports for package pkg, counted with the ones its cap dropped (API.md F7).
func warningsOf(res *Result, pkg string) Findings {
	w := Findings{Files: res.Files}
	for _, f := range res.List {
		if f.Package == pkg && f.Severity == diag.Warning {
			w.List = append(w.List, f)
		}
	}
	w.Summary.Warnings = len(w.List)
	for _, t := range res.Summary.Truncated {
		if t.Package == pkg && t.Warnings > 0 {
			w.Summary.Warnings += t.Warnings
			w.Summary.Truncated = append(w.Summary.Truncated, diag.Truncation{Package: pkg, Warnings: t.Warnings})
		}
	}
	return w
}

// unreported is moved less each W7101 whose header has one in pkg's bag, read whole (WIRE.md §6.8).
func (r *run) unreported(pkg string, moved []diag.Finding) []diag.Finding {
	if len(moved) == 0 {
		return nil
	}
	b := r.s.bag(pkg)
	b.Truncate(math.MaxInt)
	defer b.Truncate(r.p.findingLimit())
	reported := map[source.FileID]bool{}
	for _, f := range b.Findings() {
		if f.Code == diag.W7101.Def().Code {
			reported[f.Span.File] = true
		}
	}
	return slices.DeleteFunc(slices.Clone(moved), func(f diag.Finding) bool { return reported[f.Span.File] })
}

// headerSkips is, by selected package, the W7101 its tests' loads left in testBags (WIRE.md §6.8).
func (r *run) headerSkips(testBags check.Bags) map[string][]diag.Finding {
	out := map[string][]diag.Finding{}
	for _, u := range r.selected {
		if b := testBags[u.Name]; b != nil {
			out[u.Name] = slices.DeleteFunc(b.Findings(), func(f diag.Finding) bool { return f.Code != diag.W7101.Def().Code })
		}
	}
	return out
}

// uncapped is bags, each keeping every finding, so headerSkips misses no W7101 a cap would drop.
func uncapped(bags check.Bags) check.Bags {
	for _, b := range bags { //canon:unordered each bag is set alone
		b.Truncate(math.MaxInt)
	}
	return bags
}

// findingLimit is how many findings a package keeps: Options.MaxFindings, else diag's default (API.md F7).
func (p *Project) findingLimit() int {
	if n := p.opt.MaxFindings; n > 0 {
		return n
	}
	return diag.DefaultMaxFindings
}

// withWarnings is part, a package's errors, joined by w, its warnings, as its bag would keep
// them: in F2 order, the first limit, one truncation counting both (API.md F2, F7).
func withWarnings(part, w Findings, limit int) Findings {
	out := Findings{Files: part.Files, List: slices.Concat(part.List, w.List), Summary: part.Summary.Merge(w.Summary)}
	resort(out.Files, out.List)
	var over []diag.Finding
	if len(out.List) > limit {
		out.List, over = out.List[:limit], out.List[limit:]
	}
	out.Summary.Truncated = truncation(out.Summary.Truncated, over)
	return out
}

// truncation is one package's truncations ts as one, counting over too; none when nothing is dropped (API.md F7).
func truncation(ts []diag.Truncation, over []diag.Finding) []diag.Truncation {
	if len(ts)+len(over) == 0 {
		return nil
	}
	var t diag.Truncation
	for _, x := range ts {
		t.Package, t.Errors, t.Warnings = x.Package, t.Errors+x.Errors, t.Warnings+x.Warnings
	}
	for _, x := range over {
		t.Package = x.Package
		switch x.Severity {
		case diag.Error:
			t.Errors++
		case diag.Warning:
			t.Warnings++
		case diag.Runtime:
		}
	}
	return []diag.Truncation{t}
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

// testCase is a test's outcome: its failing expects, its errors outside them, then what stopped or broke it (CLI.md §3.5).
func (r *run) testCase(pkg string, f *syntax.File, d *syntax.TestDecl, run eval.TestRun, static []diag.Finding) TestCase {
	tc := TestCase{Package: pkg, Name: eval.TestName(d), Loc: r.s.set.Locate(testSpan(f, d)), Passed: !run.Failed}
	for _, x := range run.Expects {
		if !x.Passed {
			tc.Failures = append(tc.Failures, r.expectFailure(f, x))
		}
	}
	tc.Failures = append(tc.Failures, r.broke(run.Broke)...)
	tc.Failures = append(tc.Failures, r.stops(run, static, f.Span(d))...)
	return tc
}

// broke is one failure per error finding outside every expect, in F2 order, a duplicate once (EVALUATION.md §10.4, §14).
func (r *run) broke(findings []diag.Finding) []ExpectFailure {
	ls := diag.Locate(r.s.set, findings)
	all := make([]locatedFinding, len(ls))
	for i := range ls {
		all[i] = locatedFinding{f: findings[i], l: ls[i]}
	}
	slices.SortStableFunc(all, compareF2)
	all = slices.CompactFunc(all, func(a, b locatedFinding) bool { return compareF2(a, b) == 0 })
	out := make([]ExpectFailure, 0, len(all))
	for _, x := range all {
		out = append(out, ExpectFailure{Loc: r.s.set.Locate(x.f.Span), Findings: []diag.Finding{x.f}})
	}
	return out
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
