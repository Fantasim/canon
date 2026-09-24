package cli

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// runTest is `canon test [packages…] [--run <regex>]`: failures, summary, exit 1 on a failure (CLI.md §3.5).
func runTest(inv *invocation) int {
	p, err := inv.openProject()
	if err != nil {
		return inv.fail(err)
	}
	defer func() { _ = p.Close() }()
	selectors := inv.selectors(p.Root())
	res, err := p.Test(inv.ctx, canon.TestOptions{Packages: selectors, Run: inv.opt.run})
	if errors.Is(err, canon.ErrUnknownPackage) {
		err = inv.asTyped(p, selectors, err)
	}
	if err != nil {
		return inv.fail(err)
	}
	if err := inv.writeTest(res); err != nil {
		return inv.fail(err)
	}
	if res.Failed > 0 || res.Check.HasErrors() {
		return exitErrors
	}
	return exitOK
}

// writeTest prints any static error first, in check's format (its summary in text only, JSON keeping one final summary), then the test report (ADR-0004).
func (inv *invocation) writeTest(res *canon.TestResult) error {
	if inv.opt.format == formatJSON {
		return inv.writeTestJSON(res)
	}
	if res.Check.HasErrors() {
		if err := inv.writeFindings(res.Check.Findings, res.Check.Summary, res.Check.Duration); err != nil {
			return err
		}
		writeLine(inv.env.Stdout, "")
	}
	inv.writeTestText(res)
	return nil
}

// writeTestText prints failing blocks and -v's `ok` lines in test order, a blank line after each block, then the summary (CLI.md §3.5).
func (inv *invocation) writeTestText(res *canon.TestResult) {
	w := inv.env.Stdout
	okLast := false
	for _, t := range res.Tests {
		switch {
		case !t.Passed:
			writeLine(w, fmt.Sprintf(fmtTestHead, testFail, t.File, t.Line, t.Name))
			for _, f := range t.Failures {
				writeFailure(w, f)
			}
			writeLine(w, "")
			okLast = false
		case inv.opt.verbose && !inv.opt.quiet:
			writeLine(w, fmt.Sprintf(fmtTestHead, testOK, t.File, t.Line, t.Name))
			okLast = true
		}
	}
	if okLast {
		writeLine(w, "")
	}
	writeLine(w, fmt.Sprintf(fmtTestSummary, res.Passed, res.Failed, durationText(res.Duration)))
}

// writeFailure prints a failing expect with its detail lines, or what stopped the test (CLI.md §3.5).
func writeFailure(w io.Writer, f canon.ExpectFailure) {
	if f.Expect == "" {
		if f.Poisoned != "" {
			writeDetail(w, fmt.Sprintf(fmtAt, f.File, f.Line, fmt.Sprintf(fmtPoisoned, f.Poisoned)))
		}
	} else {
		writeDetail(w, fmt.Sprintf(fmtAt, f.File, f.Line, strings.Join(strings.Fields(f.Expect), wordSep)))
	}
	for _, line := range detailLines(f) {
		writeDetail(w, line)
	}
	for _, x := range f.Cause {
		writeDetail(w, locatedText(x))
	}
}

// detailLines are a failure's lines after its head, a comparison's even for empty operands (CLI.md §3.5).
func detailLines(f canon.ExpectFailure) []string {
	if f.Expect == "" {
		lines := make([]string, 0, len(f.Findings))
		for _, x := range f.Findings {
			lines = append(lines, locatedText(x))
		}
		return lines
	}
	expected, got := reportTexts(f)
	var lines []string
	if f.Op != "" {
		lines = append(lines, labeled(labelExpected, expected))
	}
	if f.Op != "" || got != "" {
		lines = append(lines, labeled(labelGot, got))
	}
	for _, x := range f.Findings {
		lines = append(lines, labeled(labelGot, findingText(x)))
	}
	return lines
}

// reportTexts are a failing expect's texts after `expected:` and `got:` (CLI.md §3.5).
func reportTexts(f canon.ExpectFailure) (expected, got string) {
	switch {
	case f.Poisoned != "":
		return "", fmt.Sprintf(fmtPoisoned, f.Poisoned)
	case f.Op != "":
		return joinWords(shownOp(f.Op), f.Expected), f.Got
	case f.Got != "":
		return "", f.Got
	case f.Outcome != "" && len(f.Findings) == 0: // a failing `passes` always captured one
		return "", gotNoFinding
	}
	return "", ""
}

// shownOp is the operator `expected:` shows: none for ==.
func shownOp(op string) string {
	if op == opEqual {
		return ""
	}
	return op
}

// joinWords joins the words that are not empty with one space.
func joinWords(words ...string) string {
	return strings.Join(slices.DeleteFunc(words, func(w string) bool { return w == "" }), wordSep)
}

// labeled is a label and its text, with no trailing space when the text is empty (API.md F9).
func labeled(label, text string) string {
	return joinWords(label, text)
}

// findingText is `<severity>[<CODE>]  <message>` (CLI.md §3.5).
func findingText(f canon.Finding) string {
	return fmt.Sprintf(fmtFindingText, f.Severity, f.Code, f.Message)
}

// locatedText is a finding's text after its `<file>:<line>`, when it has a file (CLI.md §3.5).
func locatedText(f canon.Finding) string {
	if f.File == "" {
		return findingText(f)
	}
	return fmt.Sprintf(fmtAt, f.File, f.Line, findingText(f))
}

// writeDetail writes a detail line indented two spaces, and each further line of it at the same
// indent (API.md F10).
func writeDetail(w io.Writer, text string) {
	for l := range strings.SplitSeq(text, lineBreak) {
		writeLine(w, outputIndent+l)
	}
}

// durationText is a summary's duration as API.md F15 writes it: `<n> ms` below one second,
// `<s.d> s` (rounded down) from one second on.
func durationText(d time.Duration) string {
	d = max(d, 0)
	if d < time.Second {
		return fmt.Sprintf(fmtMillis, d.Milliseconds())
	}
	return fmt.Sprintf(fmtSeconds, d.Truncate(tenthSecond).Seconds())
}
