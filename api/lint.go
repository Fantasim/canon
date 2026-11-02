package canon

import (
	"cmp"
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/lint"
)

// LintResult is Lint's (API.md L1, L11; additive to the frozen API under IMPLEMENTATION-PLAN §4).
type LintResult struct {
	Findings []Finding
	Fixes    []LintFix
	Summary  Summary
	Revision Revision
}

// LintFix is the request that fixes one finding, found at Span: complete, Base = Revision (rule L10).
type LintFix struct {
	Code string
	Span
	Edit Edit
}

// Lint returns the advisory lints W0001-W0008 of the selected packages from the analysis Check
// reads for them (rules L1-L11): it does not require a project without findings, and never
// judges a broken declaration or a value the analysis could not compute.
func (p *Project) Lint(ctx context.Context, packages ...string) (res *LintResult, err error) {
	defer recoverInternal(&err)
	s, err := p.read(ctx)
	if err != nil {
		return nil, err
	}
	a, err := analyze(ctx, s, packages)
	if err != nil {
		return nil, err
	}
	found, err := lint.Run(ctx, a, edit.NewSnapshot(a))
	if err != nil {
		return nil, apiError(err)
	}
	rev, err := p.revision(ctx, s)
	if err != nil {
		return nil, err
	}
	res = &LintResult{Findings: fromDiag(a.Files(), found.Findings), Summary: summaryOf(found.Summary), Revision: rev}
	res.Fixes = fixesOf(a.Files(), found.Fixes, rev)
	return res, nil
}

// fixesOf are fixes as complete requests based on rev, in the F2 order of their findings (rule L11):
// a finding has at most one fix, so its span and code place it.
func fixesOf(files diag.Files, fixes []lint.Fix, rev Revision) []LintFix {
	out := make([]LintFix, 0, len(fixes))
	for _, f := range fixes {
		ops := make([]Op, len(f.Ops))
		for i, op := range f.Ops {
			ops[i] = opOf(op)
		}
		out = append(out, LintFix{Code: f.Code, Span: locate(files, f.Span), Edit: Edit{Base: rev, Ops: ops}})
	}
	slices.SortStableFunc(out, func(a, b LintFix) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col), cmp.Compare(a.Code, b.Code))
	})
	return out
}
