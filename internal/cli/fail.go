package cli

import (
	"context"
	"errors"

	canon "github.com/fantasim/canonlang/api"
)

// fail reports what stopped a command and returns its exit code (CLI.md §2.5, API.md §15).
func (inv *invocation) fail(err error) int {
	var perr *canon.ProjectError
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeLine(inv.env.Stderr, msgPrefix+msgInterrupted)
		return exitInterrupted
	case errors.As(err, &perr):
		if werr := inv.writeFindings(perr.Findings, summaryOf(perr.Findings), 0); werr != nil {
			return inv.fail(werr)
		}
		return exitUsage
	case errors.Is(err, canon.ErrInternal):
		writeLine(inv.env.Stderr, msgPrefix+err.Error())
		writeLine(inv.env.Stderr, msgReportBug)
		return exitInternal
	}
	writeLine(inv.env.Stderr, msgPrefix+err.Error())
	return exitUsage
}

// summaryOf counts findings that belong to no package.
func summaryOf(findings []canon.Finding) canon.Summary {
	var s canon.Summary
	for _, f := range findings {
		if f.Severity == canon.SeverityWarning {
			s.Warnings++
		} else {
			s.Errors++
		}
	}
	return s
}

// openProject opens --project, which must hold project.canon, else the one found (CLI.md §2.1).
func (inv *invocation) openProject() (*canon.Project, error) {
	root := inv.abs(inv.opt.project)
	if inv.opt.project == "" {
		found, err := canon.FindProject(inv.env.Dir)
		if err != nil {
			return nil, err
		}
		root = found
	}
	return canon.Open(root, canon.Options{Roots: inv.opt.roots})
}
