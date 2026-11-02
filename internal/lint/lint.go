package lint

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// Result is a lint: findings in F2 order, fixes in finding order, counts with the dropped findings (API.md L1, L11).
type Result struct {
	Findings []diag.Finding
	Fixes    []Fix
	Summary  diag.Summary
}

// Fix is the edit that fixes one finding, as the operations of a request the caller bases on the revision linted (API.md L10).
type Fix struct {
	Code string
	Span source.Span
	Ops  []edit.Operation
}

// fixKey is the finding a fix belongs to.
type fixKey struct {
	code diag.Code
	span source.Span
}

// pass is one Run: the frozen analysis, what the lints share of it and the fixes they offer.
type pass struct {
	prog   *check.Program
	text   map[types.Type]bool // the records and cases a @text fn result reaches
	fixes  map[fixKey]Fix
	broken broken
	graph  graph
}

// lintFn is one lint over one package; it reports into bag and may offer fixes.
type lintFn func(p *pass, pkg *check.Package, bag *diag.Bag)

// lints are the lints Run applies, in order.
var lints = []lintFn{legacyPrefixes, markerWords, bigFiles}

// Run lints the packages a selected, one bag per package truncated at Options.MaxFindings (API.md L1, L11).
func Run(ctx context.Context, a *build.Analysis, _ *edit.Snapshot) (*Result, error) {
	prog := a.Program()
	p := &pass{prog: prog, text: textReach(prog), fixes: map[fixKey]Fix{}, broken: brokenIn(prog), graph: graphOf(prog, a.Units())}
	res := &Result{}
	for _, name := range a.Selected() {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf(fmtCancelled, name, err)
		}
		pkg := packageNamed(prog, name)
		if pkg == nil {
			continue
		}
		bag := diag.NewBag(a.Files(), name)
		bag.Truncate(a.MaxFindings())
		for _, l := range lints {
			l(p, pkg, bag)
		}
		res.Findings = append(res.Findings, bag.Findings()...)
		res.Summary = res.Summary.Merge(bag.Summary())
	}
	for _, f := range res.Findings {
		if fix, ok := p.fixes[fixKey{f.Code, f.Span}]; ok {
			res.Fixes = append(res.Fixes, fix)
		}
	}
	return res, nil
}

// packageNamed is the checked package at path, nil when the program loaded none.
func packageNamed(prog *check.Program, path string) *check.Package {
	for _, pkg := range prog.Packages {
		if pkg.Path == path {
			return pkg
		}
	}
	return nil
}

// sources are the package source files of pkg: layers and translations hold no declaration to judge.
func sources(pkg *check.Package) []*syntax.File {
	var out []*syntax.File
	for _, f := range pkg.Files {
		if f.FileKind == syntax.FileSource {
			out = append(out, f)
		}
	}
	return out
}
