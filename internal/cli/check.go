package cli

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/project"
)

// runCheck is `canon check [packages…] [--watch]` (CLI.md §3.3): every finding, then the summary; with --watch, then what changed after each change.
func runCheck(inv *invocation) int {
	p, err := inv.openProject()
	if err != nil {
		return inv.fail(err)
	}
	defer func() { _ = p.Close() }()
	events, stop, err := inv.startWatch(p)
	if err != nil {
		return inv.fail(err)
	}
	defer stop()
	selectors := inv.selectors(p.Root())
	res, err := inv.check(p, selectors)
	if err != nil {
		return inv.fail(err)
	}
	if err := inv.writeFindings(inv.shown(res.Findings), res.Summary, res.Duration); err != nil {
		return inv.fail(err)
	}
	if events != nil {
		rerun := func() (*cycleState, error) {
			next, err := inv.check(p, selectors)
			if err != nil {
				return nil, err
			}
			return inv.checkState(next), nil
		}
		return inv.watchLoop(events, inv.checkState(res), rerun)
	}
	return inv.checkExit(res.Summary)
}

// check is one run of Check over the selected packages, an unknown selector reported as typed.
func (inv *invocation) check(p *canon.Project, selectors []string) (*canon.CheckResult, error) {
	res, err := p.Check(inv.ctx, selectors...)
	if errors.Is(err, canon.ErrUnknownPackage) {
		err = inv.asTyped(p, selectors, err)
	}
	return res, err
}

// checkExit is check's exit code for its summary (CLI.md §3.3).
func (inv *invocation) checkExit(s canon.Summary) int {
	switch {
	case s.Errors > 0:
		return exitErrors
	case inv.opt.maxWarnings != unlimited && s.Warnings > inv.opt.maxWarnings:
		return exitWarnings
	}
	return exitOK
}

// shown is findings less the warnings under -q, which prints errors only (CLI.md §2.3).
func (inv *invocation) shown(findings []canon.Finding) []canon.Finding {
	if !inv.opt.quiet {
		return findings
	}
	return slices.DeleteFunc(slices.Clone(findings), func(f canon.Finding) bool { return f.Severity != canon.SeverityError })
}

// writeFindings prints findings and the summary through the API's one writer (API.md F16).
func (inv *invocation) writeFindings(findings []canon.Finding, s canon.Summary, d time.Duration) error {
	opts := canon.WriteOptions{JSON: inv.opt.format == formatJSON, Summary: s, Duration: d}
	if !inv.colored() {
		return canon.WriteFindings(inv.env.Stdout, findings, opts)
	}
	var b bytes.Buffer
	if err := canon.WriteFindings(&b, findings, opts); err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return writeText(inv.env.Stdout, inv.paint(b.String()))
}

// selectors are the arguments, a path made relative to the project root (CLI.md §2.2).
func (inv *invocation) selectors(root string) []string {
	out := make([]string, 0, len(inv.args))
	for _, arg := range inv.args {
		if !isPath(arg) {
			out = append(out, arg)
			continue
		}
		rel, err := filepath.Rel(filepath.FromSlash(root), inv.abs(arg))
		if err != nil {
			out = append(out, arg)
			continue
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(rel, project.SourceExt) {
			rel = dotSep + pathSep + rel
		}
		out = append(out, rel)
	}
	return out
}

// isPath tells a directory or file argument (`./x`, `../x`, absolute, `*.canon`) from a name.
func isPath(arg string) bool {
	slashed := filepath.ToSlash(arg)
	return strings.HasPrefix(slashed, dotSep+pathSep) || strings.HasPrefix(slashed, parentDir+pathSep) ||
		filepath.IsAbs(arg) || strings.HasSuffix(arg, project.SourceExt) || arg == dotSep || arg == parentDir
}

// asTyped names the first selector the project refuses as it was typed, not as made
// project-relative, resolving them against one listing of the packages; err stands otherwise.
func (inv *invocation) asTyped(p *canon.Project, selectors []string, err error) error {
	if len(selectors) == 1 {
		return typedError(err, inv.args[0])
	}
	pkgs, lerr := p.Packages(inv.ctx)
	if lerr != nil {
		return err
	}
	listings := make([]project.Listing, len(pkgs))
	for i, pkg := range pkgs {
		listings[i] = project.Listing{Name: pkg.Name, Dir: pkg.Dir, Files: pkg.Files}
	}
	for i, sel := range selectors {
		if _, one := project.Match(listings, sel); one != nil {
			return typedError(one, inv.args[i])
		}
	}
	return err
}

func typedError(err error, typed string) error {
	if errors.Is(err, project.ErrMixedDirectory) {
		return fmt.Errorf(fmtSelectorWhy, canon.ErrUnknownPackage, typed, project.ErrMixedDirectory)
	}
	return fmt.Errorf(fmtSelector, canon.ErrUnknownPackage, typed)
}
