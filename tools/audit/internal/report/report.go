package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/ratchet"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

type Census struct {
	Repo     *repo.Repo
	Findings []finding.Finding
	Skipped  []lane.Skip
	Enabled  map[string]bool
	Baseline ratchet.Baseline
	HasBase  bool
	Verdict  ratchet.Verdict
	PerRule  int
	Only     string
	Limits   threshold.Set
}

// printer writes through w and keeps the first error, so every Print* call site checks one
// value instead of one per line; writes after the first error are no-ops.
type printer struct {
	w   io.Writer
	err error
}

func (p *printer) f(format string, a ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, a...)
}

func (p *printer) ln(a ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintln(p.w, a...)
}

// PrintCensus prints every enabled rule with its count, zeros included, then the findings
// per rule, capped at PerRule unless a single rule was asked for.
func PrintCensus(w io.Writer, c Census) error {
	counts := map[string]int{}
	byRule := map[string][]finding.Finding{}
	for _, f := range c.Findings {
		counts[f.Rule]++
		byRule[f.Rule] = append(byRule[f.Rule], f)
	}
	p := &printer{w: w}
	censusTable(p, c, counts, freshByRule(c.Verdict))
	if p.err == nil {
		p.err = printSkips(w, c.Skipped)
	}
	censusFindings(p, c, byRule)
	return p.err
}

// censusTable prints the header line and one row per enabled rule.
func censusTable(p *printer, c Census, counts, fresh map[string]int) {
	p.f("%s: %s, %d findings\n\n", rules.ToolName, c.Repo.Name, len(c.Findings))
	p.f("%-10s %-24s %-8s %6s", "FAMILY", headerRule, "MODE", "COUNT")
	if c.HasBase {
		p.f(" %6s", "NEW")
	}
	p.ln()
	for _, rl := range rules.All {
		if !c.Enabled[rl.ID] {
			continue
		}
		mode := string(c.Repo.Mode(rl))
		if rl.Tool == rules.Mechanism {
			mode = rules.Mechanism
		}
		p.f("%-10s %-24s %-8s %6d", rl.Family, rl.ID, mode, counts[rl.ID])
		if c.HasBase {
			p.f(" %6s", dash(fresh[rl.ID]))
		}
		p.ln()
	}
}

// censusFindings prints the detail block for every rule that has findings.
func censusFindings(p *printer, c Census, byRule map[string][]finding.Finding) {
	for _, rl := range rules.All {
		fs := byRule[rl.ID]
		if len(fs) == 0 {
			continue
		}
		p.f("\n%s (%d): %s\n", rl.ID, len(fs), rl.Describe(c.Limits))
		limit := c.PerRule
		if c.Only != "" {
			limit = len(fs)
		}
		for i, f := range fs {
			if i == limit {
				p.f("  ... %d more: audit --rule %s\n", len(fs)-limit, rl.ID)
				break
			}
			p.f("  %s\n", line(f))
		}
	}
}

// checkFailed is the gate's verdict (DECISIONS 25): a ratchet failure, or any unmeasured lane.
func checkFailed(v ratchet.Verdict, skipped []lane.Skip) bool {
	return v.Failed() || len(skipped) > 0
}

// PrintCheck prints the ratchet's verdict: only what fails the gate, then one summary line.
// It returns whether the gate failed, so the caller's exit code and the printed status agree.
func PrintCheck(w io.Writer, r *repo.Repo, v ratchet.Verdict, skipped []lane.Skip) (bool, error) {
	p := &printer{w: w}
	for _, f := range v.Enforced {
		p.f("%-5s %s\n", tagEnforce, line(f))
	}
	for _, f := range v.New {
		p.f("%-5s %s\n", tagNew, line(f))
	}
	for _, g := range v.Grew {
		f := g.F
		f.Message += fmt.Sprintf(" (was %d)", g.Was)
		p.f("%-5s %s\n", tagGrew, line(f))
	}
	if p.err == nil {
		p.err = printSkips(w, skipped)
	}
	failed := checkFailed(v, skipped)
	status := statusPass
	if failed {
		status = statusFail
	}
	scope := "whole repo"
	if r.ChangedMode() {
		scope = strconv.Itoa(len(r.Changed())) + " changed files"
	}
	p.f("%s check: %s: %s (%s): %d new, %d grew, %d enforced, %d fixed, %d unmeasured\n",
		rules.ToolName, r.Name, status, scope, len(v.New), len(v.Grew), len(v.Enforced), v.Fixed, len(skipped))
	if v.Fixed > 0 {
		p.ln("  fixed findings: run `baseline --tighten` and commit " + repo.BaselineFile)
	}
	return failed, p.err
}

// PrintRaw prints one tab-separated line per finding.
func PrintRaw(w io.Writer, fs []finding.Finding) error {
	p := &printer{w: w}
	for _, f := range fs {
		p.f("%s\t%s\t%d\t%s\t%d\t%s\t%s\n", f.Rule, f.File, f.Line, f.Symbol, f.Value,
			finding.Clean(f.Message), finding.Clean(f.Fix))
	}
	return p.err
}

// PrintRules prints the rulebook, each summary with its thresholds.
func PrintRules(w io.Writer, family string, limits threshold.Set) error {
	p := &printer{w: w}
	for _, rl := range rules.All {
		if family != "" && rl.Family != family {
			continue
		}
		p.f("%-10s %-24s %-8s %s [%s]\n", rl.Family, rl.ID, rl.Mode, rl.Describe(limits), rl.Tool)
	}
	return p.err
}

// line renders one finding: rule, where, what, and the fix.
func line(f finding.Finding) string { return fmt.Sprintf("%-22s %s", f.Rule, body(f)) }

func body(f finding.Finding) string {
	var b strings.Builder
	b.WriteString(f.File)
	if f.Line > 0 {
		b.WriteString(":" + strconv.Itoa(f.Line))
	}
	if f.Symbol != "" {
		b.WriteString(" " + f.Symbol)
	}
	b.WriteString(": " + f.Message)
	if f.Fix != "" {
		b.WriteString(" -> " + f.Fix)
	}
	return b.String()
}

// printSkips lists what was not measured, so a skip never reads as a zero.
func printSkips(w io.Writer, s []lane.Skip) error {
	if len(s) == 0 {
		return nil
	}
	p := &printer{w: w}
	p.ln("\nnot measured (the census is incomplete for these):")
	for _, k := range s {
		p.f("  %s: %s\n", k.What, k.Reason)
	}
	return p.err
}

func freshByRule(v ratchet.Verdict) map[string]int {
	m := map[string]int{}
	for _, f := range v.New {
		m[f.Rule]++
	}
	for _, g := range v.Grew {
		m[g.F.Rule]++
	}
	for _, f := range v.Enforced {
		m[f.Rule]++
	}
	return m
}

func dash(n int) string {
	if n == 0 {
		return "-"
	}
	return strconv.Itoa(n)
}
