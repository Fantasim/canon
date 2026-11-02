package cli

import (
	"errors"
	"flag"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// lintFlags are canon lint's own flags (CLI.md §3.19), on top of the global ones.
func lintFlags(fs *flag.FlagSet, o *options) {
	fs.BoolVar(&o.fix, flagFix, o.fix, usageFix)
}

// runLint is `canon lint [packages…] [--fix]` (CLI.md §3.19): fix lines, findings, summary.
func runLint(inv *invocation) int {
	start := time.Now()
	inv.opt.format = formatJSON // CLI.md §3.19: whatever --format says (DECISIONS 274)
	p, err := inv.openProject()
	if err != nil {
		return inv.fail(err)
	}
	defer func() { _ = p.Close() }()
	res, err := inv.lint(p, inv.selectors(p.Root()))
	if err != nil {
		return inv.fail(err)
	}
	if inv.opt.fix {
		for _, f := range res.Fixes {
			if err := inv.writeJSONLine(lintFixLine{Fix: lintFixOf(f)}); err != nil {
				return inv.fail(err)
			}
		}
	}
	if err := inv.writeFindings(res.Findings, res.Summary, time.Since(start)); err != nil {
		return inv.fail(err)
	}
	if inv.opt.maxWarnings != unlimited && res.Summary.Warnings > inv.opt.maxWarnings {
		return exitWarnings
	}
	return exitOK
}

// lint is one Lint of the selected packages, an unknown selector reported as typed.
func (inv *invocation) lint(p *canon.Project, selectors []string) (*canon.LintResult, error) {
	res, err := p.Lint(inv.ctx, selectors...)
	if errors.Is(err, canon.ErrUnknownPackage) {
		err = inv.asTyped(p, selectors, err)
	}
	return res, err
}

// lintFixLine is the line canon lint --fix prints per fix (CLI.md §3.19, IMPLEMENTATION-PLAN §8.1).
type lintFixLine struct {
	Fix lintFixBody `json:"fix"`
}

// lintFixBody is its members, in a fixed order; Request is a complete request for canon edit (§3.15).
type lintFixBody struct {
	Code    string     `json:"code"`
	File    string     `json:"file"`
	Line    int        `json:"line"`
	Col     int        `json:"col"`
	Request canon.Edit `json:"request"`
}

func lintFixOf(f canon.LintFix) lintFixBody {
	return lintFixBody{Code: f.Code, File: f.File, Line: f.Line, Col: f.Col, Request: f.Edit}
}
