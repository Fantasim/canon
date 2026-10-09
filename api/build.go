package canon

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/views"
	"github.com/fantasim/canonlang/internal/workspace"
)

// BuildOptions selects what Build does (API.md §13.1).
type BuildOptions struct {
	Packages []string
	Targets  []Target
	Check    bool // write nothing; report what would change (CLI --check)
	Adopt    []string
	OnlyRoot string // a consumer root: write only the outputs under it, nothing of the project (CLI --only-root, B1c)
}

// Output is one emitted file.
type Output struct {
	Path    string
	Target  Target
	Package string
	Status  OutputStatus
}

// LockChange lists the canon.lock lines a build appended (or would append).
type LockChange struct {
	Package string
	File    string
	Lines   []string
}

// BuildResult is the result of Build.
type BuildResult struct {
	Check   *CheckResult
	Outputs []Output
	Lock    []LockChange
	Stale   bool
}

// Build checks the selected packages and, without errors, writes their outputs (rules B1, B2).
// A writing Build (Check false) is the project's one writer (S9) and returns the revision of
// the snapshot it publishes (S10); Build with Check true is a read (S8) and takes no lock.
func (p *Project) Build(ctx context.Context, o BuildOptions) (res *BuildResult, err error) {
	defer recoverInternal(&err)
	targets, err := irTargets(o.Targets)
	if err != nil {
		if _, rerr := p.read(ctx); rerr != nil { // O6 first
			return nil, rerr
		}
		return nil, err
	}
	opts := build.BuildOptions{Packages: o.Packages, Targets: targets, Adopt: o.Adopt, Check: o.Check, OnlyRoot: o.OnlyRoot}
	start := time.Now()
	s, r, err := p.runBuild(ctx, opts)
	if err != nil {
		return nil, onlyRootError(err, o)
	}
	rev, err := p.revision(context.WithoutCancel(ctx), s) // a done ctx never drops a write's result (S10)
	if err != nil {
		return nil, err
	}
	out := buildResultOf(r, time.Since(start))
	out.Check.Revision = rev
	return out, nil
}

// runBuild runs a build: a shared read on the current snapshot with Check, else the project's
// writer, and returns the snapshot the result belongs to (rules S8-S10).
func (p *Project) runBuild(ctx context.Context, opts build.BuildOptions) (*workspace.Snapshot, *build.BuildResult, error) {
	if opts.Check {
		s, err := p.read(ctx)
		if err != nil {
			return nil, nil, err
		}
		key := workspace.Key(workspace.OpBuild, opts.Packages, append([]string{targetKey(opts.Targets), opts.OnlyRoot}, opts.Adopt...)...)
		r, err := share(ctx, s, key, func(ctx context.Context) (*build.BuildResult, error) { return s.Build().Build(ctx, opts) })
		return s, r, err
	}
	var r *build.BuildResult
	next, err := p.workspace().Write(ctx, workspace.CauseEdit, func(ctx context.Context, s *workspace.Snapshot) error {
		var err error
		r, err = s.Build().Build(ctx, opts)
		return err
	})
	if err != nil {
		return nil, nil, apiError(err)
	}
	return next, r, nil
}

// onlyRootError is a refused OnlyRoot as *ValueError (rule V1, DECISIONS 343): a root that is not
// a consumer root of the project, or one given with Adopt; any other error as it is.
func onlyRootError(err error, o BuildOptions) error {
	switch {
	case errors.Is(err, build.ErrNotConsumerRoot):
		return &ValueError{Op: -1, Expected: expectedConsumerRoot, Got: fmt.Sprintf(fmtQuoted, o.OnlyRoot)}
	case errors.Is(err, build.ErrOnlyRootAdopt):
		return &ValueError{Op: -1, Expected: expectedNoAdopt, Got: fmt.Sprintf(fmtAdoptGot, o.Adopt)}
	}
	return err
}

// targetKey is the selected targets as one part of a shared build's key.
func targetKey(targets []ir.Target) string {
	var b strings.Builder
	for _, t := range targets {
		b.WriteString(string(apiTarget(t)))
		b.WriteString(textSep)
	}
	return b.String()
}

// buildResultOf converts a build's result into the API's form (rule B1); a lock is listed only
// when it gained lines (DECISIONS 201).
func buildResultOf(r *build.BuildResult, d time.Duration) *BuildResult {
	out := &BuildResult{Check: checkResultOf(r.Result, d), Stale: r.Stale}
	for _, o := range r.Outputs {
		out.Outputs = append(out.Outputs, Output{Path: o.Path, Target: apiTarget(o.Target), Package: o.Package, Status: apiStatus(o.Status)})
	}
	for _, l := range r.Locks {
		if len(l.Lines) == 0 {
			continue
		}
		out.Lock = append(out.Lock, LockChange{Package: l.Package, File: l.Path, Lines: slices.Clone(l.Lines)})
	}
	return out
}

// targetNames maps every ir.Target to the API's Target, table-driven both ways (SPEC §14).
var targetNames = [...]Target{
	ir.TargetGo: TargetGo, ir.TargetCpp: TargetCpp, ir.TargetTS: TargetTS, ir.TargetJSON: TargetJSON, ir.TargetView: TargetView, ir.TargetText: TargetText,
}

// irTargets is ts translated to ir.Target; an unknown value is *ValueError (rule V1, DECISIONS
// 201): a list must never widen to every target because one entry did not fit.
func irTargets(ts []Target) ([]ir.Target, error) {
	if len(ts) == 0 {
		return nil, nil
	}
	out := make([]ir.Target, 0, len(ts))
	for _, t := range ts {
		it, ok := irTarget(t)
		if !ok {
			return nil, &ValueError{Op: -1, Expected: expectedTargets, Got: fmt.Sprintf(fmtQuoted, t)}
		}
		out = append(out, it)
	}
	return out, nil
}

func irTarget(t Target) (ir.Target, bool) {
	for i, name := range targetNames {
		if name == t {
			return ir.Target(i), true
		}
	}
	return 0, false
}

// apiTarget is t as the API names it.
func apiTarget(t ir.Target) Target {
	if int(t) < len(targetNames) {
		return targetNames[t]
	}
	return ""
}

// statusNames maps every build.Status to the API's OutputStatus (API.md §13.1).
var statusNames = [...]OutputStatus{
	build.StatusWritten: OutputWritten, build.StatusUnchanged: OutputUnchanged,
	build.StatusAdopted: OutputAdopted, build.StatusStale: OutputStale,
}

// apiStatus is s as the API names it.
func apiStatus(s build.Status) OutputStatus {
	if int(s) < len(statusNames) {
		return statusNames[s]
	}
	return ""
}

// TestOptions selects tests.
type TestOptions struct {
	Packages []string
	Run      string // RE2 matched against test names; empty runs all
}

// ExpectFailure is a failing expect, or with Expect "" what stopped the test (rule B3; log-2026-09-24 "canon test review calls").
type ExpectFailure struct {
	Span
	Expect   string
	Outcome  string // passes, fails or warns; "" for `expect c` and a stop
	Op       string // a comparison's operator when both operands were evaluated; Expected and Got are then its operands
	Expected string
	Got      string // a comparison's left operand, or the value of any other Boolean
	Poisoned string // the poisoned top-level value read
	Findings []Finding
	Cause    []Finding // the errors that poisoned it
}

// TestCase is the result of one test block.
type TestCase struct {
	Package string
	Name    string
	Span
	Passed   bool
	Failures []ExpectFailure
}

// TestResult is the result of Test.
type TestResult struct {
	Check    *CheckResult // phases 1-2's error findings of the loaded packages (ADR-0004)
	Tests    []TestCase
	Passed   int
	Failed   int
	Duration time.Duration
}

// Test runs the selected packages' tests o.Run matches, in (package, file, line) order; a bad Run is *ValueError (rule B3, EVALUATION.md §10).
func (p *Project) Test(ctx context.Context, o TestOptions) (res *TestResult, err error) {
	defer recoverInternal(&err)
	s, err := p.read(ctx)
	if err != nil {
		return nil, err
	}
	match, err := runPattern(o.Run)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	key := workspace.Key(workspace.OpTest, o.Packages, o.Run)
	r, err := share(ctx, s, key, func(ctx context.Context) (*build.TestResult, error) { return s.Build().Test(ctx, o.Packages, match) })
	if err != nil {
		return nil, err
	}
	res = testResultOf(r, time.Since(start))
	if res.Check.Revision, err = p.revision(ctx, s); err != nil {
		return nil, err
	}
	return res, nil
}

// runPattern is Run compiled, nil when empty; an invalid one is *ValueError (log-2026-09-24 "canon test review calls").
func runPattern(run string) (*regexp.Regexp, error) {
	if run == "" {
		return nil, nil
	}
	re, err := regexp.Compile(run)
	if err != nil {
		return nil, &ValueError{Op: -1, Expected: expectedPattern, Got: fmt.Sprintf(fmtQuoted, run), Detail: err.Error()}
	}
	return re, nil
}

// testResultOf converts canon test's run into the API's form, counting its outcomes (rule B3).
func testResultOf(r *build.TestResult, d time.Duration) *TestResult {
	out := &TestResult{Check: checkResultOf(r.Static, d), Duration: d}
	for _, t := range r.Tests {
		tc := TestCase{Package: t.Package, Name: t.Name, Span: spanOf(t.Loc), Passed: t.Passed}
		for _, f := range t.Failures {
			tc.Failures = append(tc.Failures, ExpectFailure{
				Span: spanOf(f.Loc), Expect: f.Expect, Outcome: f.Outcome, Op: f.Op, Expected: f.Expected, Got: f.Got,
				Poisoned: f.Poisoned, Findings: fromDiag(r.Files, f.Findings), Cause: fromDiag(r.Files, f.Cause),
			})
		}
		if t.Passed {
			out.Passed++
		} else {
			out.Failed++
		}
		out.Tests = append(out.Tests, tc)
	}
	return out
}

// Format returns the canonical layout of one .canon file, project.canon's when filename's base
// names it (rule T1); a file that does not parse is a *SyntaxError with its findings.
func Format(filename string, src []byte) (out []byte, err error) {
	defer recoverInternal(&err)
	set := &source.FileSet{}
	f, err := set.Add(filename, filename, src)
	if err != nil {
		return nil, err
	}
	kind := syntax.FileSource
	if base := path.Base(filepath.ToSlash(filename)); base == project.FileName || base == project.LocalFileName { // T1
		kind = syntax.FileProject
	}
	bag := diag.NewBag(set, "")
	out, err = format.Source(f, kind, bag)
	if errors.Is(err, format.ErrSyntax) {
		return nil, &SyntaxError{Findings: fromDiag(set, bag.Findings())}
	}
	return out, err
}

// FormatJSONSource returns the canonical source layout of a JSON file, keys in their order
// (rule T2, FORMATTER.md 14.1); input that is not JSON, not UTF-8 or repeats a key is a
// *SyntaxError with its finding.
func FormatJSONSource(src []byte) (out []byte, err error) {
	defer recoverInternal(&err)
	set := &source.FileSet{}
	f, err := set.Add("", "", src)
	if err != nil {
		return nil, err
	}
	bag := diag.NewBag(set, "")
	root, err := jsonsrc.Parse(f, bag)
	switch {
	case err == nil:
		return jsonsrc.Format(root), nil
	case errors.Is(err, jsonsrc.ErrEncoding):
		load.ReportEncoding(bag, "", src, err)
	case !errors.Is(err, jsonsrc.ErrSyntax) && !errors.Is(err, jsonsrc.ErrDuplicateKey):
		return nil, internalError(err)
	}
	return nil, &SyntaxError{Findings: fromDiag(set, bag.Findings())}
}

// VersionInfo describes the compiler and the formats it produces (API.md §14).
type VersionInfo struct {
	Compiler    string
	Languages   []string
	Fingerprint string
	ViewModel   string
	Lock        string
	Commit      string
}

// Version returns the compiler's version information; Commit follows rule T3.
func Version() VersionInfo {
	return VersionInfo{
		Compiler:    build.CompilerVersion,
		Languages:   []string{languageVersion},
		Fingerprint: fingerprintFormat,
		ViewModel:   views.SchemaVersion,
		Lock:        lockFormat,
		Commit:      buildCommit(),
	}
}

// buildCommit is the revision of the compiler inside the running binary (API.md §14).
func buildCommit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return commitOf(info)
}

// commitOf is the compiler's VCS revision in info, "" when unknown (rule T3).
func commitOf(info *debug.BuildInfo) string {
	if info.Main.Path == modulePath {
		settings := map[string]string{}
		for _, s := range info.Settings {
			settings[s.Key] = s.Value
		}
		if settings[vcsModifiedKey] == trueText {
			return ""
		}
		return settings[vcsRevisionKey]
	}
	for _, dep := range info.Deps {
		if dep.Path == modulePath && dep.Replace == nil {
			return pseudoRevision(dep.Version)
		}
	}
	return ""
}

// pseudoRevision is the revision prefix a Go pseudo-version ends with
// (vX.Y.Z-yyyymmddhhmmss-abcdefabcdef, or with -0. or -pre.0. before the time), "" for any
// other version.
func pseudoRevision(version string) string {
	i := strings.LastIndex(version, pseudoSep)
	if i < 0 {
		return ""
	}
	rev, rest := version[i+len(pseudoSep):], version[:i]
	if len(rev) != pseudoRevLen || len(rest) < pseudoTimeLen || !isDigits(rev, hexBase) || strings.ToLower(rev) != rev {
		return ""
	}
	stamp, before := rest[len(rest)-pseudoTimeLen:], rest[:len(rest)-pseudoTimeLen]
	if !isDigits(stamp, decimalBase) || !strings.ContainsAny(before, pseudoStampSeps) {
		return ""
	}
	return rev
}

// isDigits reports s made only of digits of base: no sign, no prefix, no separator.
func isDigits(s string, base int) bool {
	_, err := strconv.ParseUint(s, base, pseudoBits)
	return err == nil
}
