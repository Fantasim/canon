package progen_test

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// baselineKey is a finding's identity for baseline comparison: its exact code and position, not
// its text (DECISIONS 200).
type baselineKey struct {
	code      diag.Code
	path      string
	line, col int
}

// keyOf is f's baseline identity.
func keyOf(f progen.Finding) baselineKey { return baselineKey{f.Code, f.Path, f.Line, f.Col} }

// severityOf is code's severity per the registry (ERRORS.md), diag.Error when code is unknown.
func severityOf(code diag.Code) diag.Severity {
	for i := range diag.Registry {
		if diag.Registry[i].Code == code {
			return diag.Registry[i].Severity
		}
	}
	return diag.Error
}

// disqualifies reports fs holding an error-severity finding: it disqualifies a package from the
// clean corpus, unlike a warning, which becomes that package's own baseline instead.
func disqualifies(fs []progen.Finding) bool {
	for _, f := range fs {
		if severityOf(f.Code) == diag.Error {
			return true
		}
	}
	return false
}

// baselineOf is the exact findings pkgs already report unmutated, keyed for lookup: a mutation's
// judgement treats one of these as neither its hit nor an extra (DECISIONS 200).
func (c *corpus) baselineOf(pkgs []string) map[baselineKey]bool {
	out := map[baselineKey]bool{}
	for _, pkg := range pkgs {
		for _, f := range c.baseline[pkg] {
			out[keyOf(f)] = true
		}
	}
	return out
}
