package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/project"
)

// runCheck is `canon check [packages…]` (CLI.md §3.3): every finding, then the summary.
func runCheck(inv *invocation) int {
	p, err := inv.openProject()
	if err != nil {
		return inv.fail(err)
	}
	defer func() { _ = p.Close() }()
	selectors := inv.selectors(p.Root())
	res, err := p.Check(inv.ctx, selectors...)
	if errors.Is(err, canon.ErrUnknownPackage) {
		err = inv.asTyped(p, selectors, err)
	}
	if err != nil {
		return inv.fail(err)
	}
	shown := res.Findings
	if inv.opt.quiet {
		shown = slices.DeleteFunc(slices.Clone(shown), func(f canon.Finding) bool { return f.Severity != canon.SeverityError })
	}
	if err := inv.writeFindings(shown, res.Summary, res.Duration); err != nil {
		return inv.fail(err)
	}
	switch {
	case res.Summary.Errors > 0:
		return exitErrors
	case inv.opt.maxWarnings != unlimited && res.Summary.Warnings > inv.opt.maxWarnings:
		return exitWarnings
	}
	return exitOK
}

// writeFindings prints findings and the summary through the API's one writer (API.md F16).
func (inv *invocation) writeFindings(findings []canon.Finding, s canon.Summary, d time.Duration) error {
	opts := canon.WriteOptions{JSON: inv.opt.format == formatJSON, Summary: s, Duration: d}
	return canon.WriteFindings(inv.env.Stdout, findings, opts)
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
