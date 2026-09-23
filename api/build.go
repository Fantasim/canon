package canon

import (
	"context"
	"runtime/debug"
	"time"
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
func (p *Project) Build(ctx context.Context, o BuildOptions) (*BuildResult, error) {
	return nil, errUnimplemented()
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

// Version returns the compiler's version information.
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

// buildCommit is the vcs.revision the go command stamped into the binary, if any.
func buildCommit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == vcsRevisionKey {
			return s.Value
		}
	}
	return ""
}
