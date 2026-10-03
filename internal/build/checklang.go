package build

import (
	"cmp"
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/render"
)

// localized puts res's named-check messages in Options.Lang (translate).
func (r *run) localized(ctx context.Context, res *Result) error {
	return r.translate(ctx, res.Files, res.List, r.p.opt.Lang)
}

// translate puts each named one-line check's finding of list in lang where translated, rendered
// as the view model's messages are, then sorts list again; the rest stays in the source language,
// and the bags, which the view model reads, are untouched (I18N.md B5, K8, T2, T4; DECISIONS 281).
func (r *run) translate(ctx context.Context, files diag.Files, list []diag.Finding, lang string) error {
	if len(list) == 0 || lang == "" || lang == r.s.proj.SourceLanguage() || len(r.failed.runs) == 0 {
		return nil
	}
	var tr *render.Renderer // made on the first finding of a named check: the rest pay nothing
	restore := func() error { return nil }
	translated := false
	for i, f := range list {
		c, self, ok := r.failed.find(f)
		if !ok {
			continue
		}
		if tr == nil {
			scratch := r.throwawayBags()
			restore = r.hostAside(scratch)
			tr = r.messageRenderer(ctx, scratch)
		}
		if msg, ok := tr.Messages(f.Package, c, self, []string{lang})[lang]; ok {
			list[i] = f.Restated(r.s.set, msg)
			translated = translated || list[i].Message != f.Message
		}
	}
	err := restore()
	if translated {
		resort(files, list)
	}
	return interrupted(ctx, err)
}

// messageRenderer renders check messages as views.Build does, reading values aside into scratch.
func (r *run) messageRenderer(ctx context.Context, scratch check.Bags) *render.Renderer {
	force := func(pkg, name string) (value.Value, bool) {
		return r.ev.ForceAside(ctx, eval.Root{Pkg: pkg, Name: name}, scratch)
	}
	return render.New(ctx, render.Input{
		Program: r.prog, Index: control.NewIndex(r.prog, r.s.proj.Studio.Path),
		Texts: encode.NewTexts(encode.Catalogues(r.texts)), Colls: encode.NewColls(force), Eval: viewEval{r.ev},
	})
}

// locatedFinding is a finding with its resolved location, which the F2 order reads.
type locatedFinding struct {
	f diag.Finding
	l diag.Located
}

// resort sorts each package's run of list again by API.md F2, ties in the bag's order.
func resort(files diag.Files, list []diag.Finding) {
	ls := diag.Locate(files, list)
	all := make([]locatedFinding, len(ls))
	for i := range ls {
		all[i] = locatedFinding{f: list[i], l: ls[i]}
	}
	for start := 0; start < len(all); {
		end := start + 1
		for end < len(all) && all[end].f.Package == all[start].f.Package {
			end++
		}
		slices.SortStableFunc(all[start:end], compareF2)
		start = end
	}
	for i, x := range all {
		list[i] = x.f
	}
}

// compareF2 is API.md F2's key: file bytes, line, column, code, message; no file first.
func compareF2(a, b locatedFinding) int {
	return cmp.Or(
		cmp.Compare(a.l.Loc.Path, b.l.Loc.Path),
		cmp.Compare(a.l.Loc.Line, b.l.Loc.Line),
		cmp.Compare(a.l.Loc.Col, b.l.Loc.Col),
		cmp.Compare(a.l.Code, b.l.Code),
		cmp.Compare(a.l.Message, b.l.Message),
	)
}
