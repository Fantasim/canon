package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

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
		inv.reportError(err)
		return exitInternal
	case errors.Is(err, canon.ErrNoValue):
		return inv.poisoned(err)
	}
	writeLine(inv.env.Stderr, msgPrefix+err.Error())
	return exitUsage
}

// reportError writes err's message to stderr, the bug-report line after an internal error; the findings of a project error are not its message, and an interrupt is Main's to report.
func (inv *invocation) reportError(err error) {
	var perr *canon.ProjectError
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded), errors.As(err, &perr):
		return
	case errors.Is(err, canon.ErrInternal):
		writeLine(inv.env.Stderr, msgPrefix+err.Error())
		writeLine(inv.env.Stderr, msgReportBug)
	default:
		writeLine(inv.env.Stderr, msgPrefix+err.Error())
	}
}

// poisoned reports a value with none: its findings, no summary, the error, exit 1 (API.md R6).
func (inv *invocation) poisoned(err error) int {
	var perr *canon.PathError
	if errors.As(err, &perr) && len(perr.Findings) > 0 {
		if werr := inv.writeFindingsOnly(perr.Findings); werr != nil {
			return inv.fail(werr)
		}
	}
	writeLine(inv.env.Stderr, msgPrefix+err.Error())
	return exitErrors
}

// writeFindingsOnly prints findings through the API's one writer (F16) without its summary line;
// in text, the blank line before it goes too (F14).
func (inv *invocation) writeFindingsOnly(findings []canon.Finding) error {
	var b bytes.Buffer
	if err := canon.WriteFindings(&b, findings, canon.WriteOptions{JSON: inv.opt.format == formatJSON}); err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return writeText(inv.env.Stdout, inv.withoutSummary(b.String()))
}

// withoutSummary is rendered findings less their summary line and, in text, the blank line before it (API.md F14).
func (inv *invocation) withoutSummary(rendered string) string {
	out := strings.TrimSuffix(rendered, lineBreak)
	out = out[:strings.LastIndex(out, lineBreak)+1]
	if inv.opt.format != formatJSON {
		out = strings.TrimSuffix(out, lineBreak)
	}
	return out
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
	return inv.openProjectFS(nil)
}

// openProjectFS is openProject reading through fsys, the OS's when nil.
func (inv *invocation) openProjectFS(fsys canon.FS) (*canon.Project, error) {
	root, err := inv.projectRoot()
	if err != nil {
		return nil, err
	}
	return canon.Open(root, canon.Options{Roots: inv.opt.roots, Layers: inv.opt.layers, FS: fsys})
}

// projectRoot is --project, else the directory holding project.canon at or above the current one (CLI.md §2.1).
func (inv *invocation) projectRoot() (string, error) {
	if inv.opt.project != "" {
		return inv.abs(inv.opt.project), nil
	}
	return canon.FindProject(inv.env.Dir)
}
