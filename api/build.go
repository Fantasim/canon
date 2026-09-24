package canon

import (
	"context"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/ir"
)

// BuildOptions selects what Build does (API.md §13.1).
type BuildOptions struct {
	Packages []string
	Targets  []Target
	Check    bool // write nothing; report what would change (CLI --check)
	Adopt    []string
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
func (p *Project) Build(ctx context.Context, o BuildOptions) (res *BuildResult, err error) {
	defer recoverInternal(&err)
	b, err := p.open()
	if err != nil {
		return nil, err
	}
	start := time.Now()
	r, err := b.Build(ctx, build.BuildOptions{
		Packages: o.Packages, Targets: irTargets(o.Targets), Adopt: o.Adopt, Check: o.Check,
	})
	if err != nil {
		return nil, apiError(err)
	}
	p.setRevision(r.Revision)
	return buildResultOf(r, time.Since(start)), nil
}

// buildResultOf converts a build's result into the API's form (rule B1).
func buildResultOf(r *build.BuildResult, d time.Duration) *BuildResult {
	out := &BuildResult{Check: checkResultOf(r.Result, d), Stale: r.Stale}
	for _, o := range r.Outputs {
		out.Outputs = append(out.Outputs, Output{Path: o.Path, Target: apiTarget(o.Target), Package: o.Package, Status: apiStatus(o.Status)})
	}
	for _, l := range r.Locks {
		out.Lock = append(out.Lock, LockChange{Package: l.Package, File: l.Path, Lines: l.Lines})
	}
	return out
}

// targetNames maps every ir.Target to the API's Target, table-driven both ways (SPEC §14).
var targetNames = [...]Target{
	ir.TargetGo: TargetGo, ir.TargetCpp: TargetCpp, ir.TargetTS: TargetTS, ir.TargetJSON: TargetJSON, ir.TargetView: TargetView,
}

// irTargets is ts translated to ir.Target, an unknown value dropped (SPEC §14).
func irTargets(ts []Target) []ir.Target {
	if len(ts) == 0 {
		return nil
	}
	out := make([]ir.Target, 0, len(ts))
	for _, t := range ts {
		for i, name := range targetNames {
			if name == t {
				out = append(out, ir.Target(i))
				break
			}
		}
	}
	return out
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

// ExpectFailure is one failing expect statement (rule B3).
type ExpectFailure struct {
	Span
	Expect   string
	Expected string
	Got      string
	Findings []Finding
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
	Tests    []TestCase
	Passed   int
	Failed   int
	Duration time.Duration
}

// Test runs the test blocks of the selected packages (rule B3).
func (p *Project) Test(ctx context.Context, o TestOptions) (*TestResult, error) {
	return nil, errUnimplemented()
}

// Format returns the canonical layout of one .canon file (rule T1).
func Format(filename string, src []byte) ([]byte, error) {
	return nil, errUnimplemented()
}

// FormatJSONSource returns the canonical source layout of a JSON file (rule T2).
func FormatJSONSource(src []byte) ([]byte, error) {
	return nil, errUnimplemented()
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
		Compiler:    compilerVersion,
		Languages:   []string{languageVersion},
		Fingerprint: fingerprintFormat,
		ViewModel:   viewModelFormat,
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
