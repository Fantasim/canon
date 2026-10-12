package build

import (
	"cmp"
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
)

// Source is Result's findings in the source language, F2 order, read-only (DECISIONS 281, 345).
func (a *Analysis) Source() []diag.Finding { return a.source }

// InOrder is a copy of findings, this analysis's in the source language, each named check's
// message in lang, "" for Options.Lang, the order kept (API.md R6a).
func (a *Analysis) InOrder(ctx context.Context, findings []diag.Finding, lang string) ([]diag.Finding, error) {
	out := slices.Clone(findings)
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := a.r.restate(ctx, out, cmp.Or(lang, a.r.p.opt.Lang))
	return out, err
}

// OwnIn is Own with the named checks' messages in lang, "" for Options.Lang: the same findings and
// counts, only their messages rendered again from the kept analysis (API.md E18, R3a).
func (a *Analysis) OwnIn(ctx context.Context, lang string) (*Result, error) {
	own := a.Own()
	if lang == "" || lang == a.r.p.opt.Lang {
		return own, nil
	}
	list, err := a.Localized(ctx, a.ownOf(a.source), lang)
	if err != nil {
		return nil, err
	}
	own.List = list
	return own, nil
}

// CauseIn is Cause with the named checks' messages in lang, "" for Options.Lang, the cause first (API.md R6, R6a).
func (a *Analysis) CauseIn(ctx context.Context, root eval.Root, lang string) ([]diag.Finding, error) {
	found, err := a.Cause(ctx, root)
	if err != nil || len(found) == 0 {
		return found, err
	}
	return a.InOrder(ctx, found, lang)
}
