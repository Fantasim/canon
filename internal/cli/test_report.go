package cli

import canon "github.com/fantasim/canonlang/api"

// testLine is canon test's JSON line for one test (CLI.md §3.5, IMPLEMENTATION-PLAN.md §8.1).
type testLine struct {
	Test struct {
		Package  string        `json:"package"`
		Name     string        `json:"name"`
		File     string        `json:"file"`
		Line     int           `json:"line"`
		Status   string        `json:"status"`
		Failures []failureLine `json:"failures"`
	} `json:"test"`
}

// failureLine is one failing expect, or what stopped the test (its expect is then "").
type failureLine struct {
	File     string          `json:"file"`
	Line     int             `json:"line"`
	Col      int             `json:"col"`
	Expect   string          `json:"expect"`
	Expected string          `json:"expected"`
	Got      string          `json:"got"`
	Findings []canon.Finding `json:"findings"`
}

func newTestLine(t canon.TestCase) testLine {
	var l testLine
	l.Test.Package, l.Test.Name, l.Test.File, l.Test.Line = t.Package, t.Name, t.File, t.Line
	l.Test.Status = statusFail
	if t.Passed {
		l.Test.Status = statusPass
	}
	l.Test.Failures = make([]failureLine, 0, len(t.Failures))
	for _, f := range t.Failures {
		expected, got := reportTexts(f)
		l.Test.Failures = append(l.Test.Failures, failureLine{
			File: f.File, Line: f.Line, Col: f.Col, Expect: f.Expect, Expected: expected, Got: got,
			Findings: append(append(make([]canon.Finding, 0, len(f.Findings)+len(f.Cause)), f.Findings...), f.Cause...),
		})
	}
	return l
}

// testSummary is canon test's final JSON line.
type testSummary struct {
	Summary struct {
		Passed int `json:"passed"`
		Failed int `json:"failed"`
		Ms     int `json:"ms"`
	} `json:"summary"`
}

// writeTestJSON prints the static errors (ADR-0004), then one line per test, only the failing ones under -q, then the summary.
func (inv *invocation) writeTestJSON(res *canon.TestResult) error {
	if res.Check.HasErrors() {
		for _, f := range res.Check.Findings {
			if err := inv.writeJSONLine(f); err != nil {
				return err
			}
		}
	}
	for _, t := range res.Tests {
		if t.Passed && inv.opt.quiet {
			continue
		}
		if err := inv.writeJSONLine(newTestLine(t)); err != nil {
			return err
		}
	}
	var s testSummary
	s.Summary.Passed, s.Summary.Failed = res.Passed, res.Failed
	s.Summary.Ms = int(max(res.Duration, 0).Milliseconds())
	return inv.writeJSONLine(s)
}
