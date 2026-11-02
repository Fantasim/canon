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
	ctx       context.Context // the Run's: the lints that evaluate a default stop with it
	a         *build.Analysis
	prog      *check.Program
	snap      *edit.Snapshot
	text      edit.Reached // the records and cases a @text fn result reaches
	output    edit.Reached // those an output reaches: a data emit or a @text result (API.md §5.7, rule L5)
	wire      edit.Reached // those a load or an output reaches (rule L3)
	fixes     map[fixKey]Fix
	broken    broken
	graph     graph
	places    *edit.Placer          // where @files templates place entries, made on first use (placer)
	defines   map[string][]string   // the macros headers define with the files that do, made on first use (headerDefines)
	following map[check.Object]bool // the lets whose value the enum-names lint is reading (letEnums)
}

// lintFn is one lint over one package; it reports into bag and may offer fixes.
type lintFn func(p *pass, pkg *check.Package, bag *diag.Bag)

// lints are the lints Run applies, in order.
var lints = []lintFn{legacyPrefixes, numberedSiblings, enumNames, authoringNotes, markerWords, entryFiles, bigFiles, textDefines}

// Run lints the packages a selected, one bag per package truncated at Options.MaxFindings (API.md L1, L11).
func Run(ctx context.Context, a *build.Analysis, s *edit.Snapshot) (*Result, error) {
	prog := a.Program()
	p := &pass{
		ctx: ctx, a: a, prog: prog, snap: s, text: s.TextReach(), output: s.OutputReach(), wire: s.WireReach(),
		fixes: map[fixKey]Fix{}, following: map[check.Object]bool{}, broken: brokenIn(prog), graph: graphOf(prog, a.Units()),
	}
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
