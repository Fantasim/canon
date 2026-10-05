package rules

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// Package runs stage D: the package checks of one package, in order (EVALUATION.md §8.2).
func (r *Runner) Package(ctx context.Context, pkg *check.Package) error {
	bag, err := r.bagOf(pkg.Path)
	if err != nil {
		return err
	}
	for _, obj := range pkg.Decls {
		c, ok := obj.Decl().(*syntax.CheckDecl)
		if obj.Kind() != check.ObjCheck || !ok || r.isBroken(obj) || ctx.Err() != nil {
			continue
		}
		run := r.ev.Run(ctx, c, nil, "")
		if run.Aborted {
			continue
		}
		file := obj.File()
		if run.Failed {
			at := keyword(file, c)
			r.decorate(oneLine(c, at, run.Message), file, c).Report(bag)
			r.tell(c, nil, "", run)
		}
		r.blockReports(c, file, run.Reports, bag, nil)
	}
	return nil
}

// report places an instance check run's findings at at, nil: the value visited (EVALUATION.md §8.3).
func (t *traversal) report(c *syntax.CheckDecl, run Run, rec *value.Record, at *verify.Path) {
	if run.Aborted {
		return
	}
	bag, file := t.bag, t.files[c]
	if run.Failed {
		v, onField := Placed(c, rec)
		site := verify.SiteOf(v)
		b := t.decorate(oneLine(c, site.Span, run.Message), file, c)
		if c.At == nil {
			b.Reads(t.reads(c, rec))
		}
		if t.under != nil { // a precomputed result's: no path, its frame (DECISIONS 324)
			site.ReportUnder(b, *t.under, bag, t.hider())
			t.tell(c, rec, "", run)
		} else {
			p := t.placed(at, onField, c)
			site.Report(b, p, bag)
			t.tell(c, rec, p.String(), run)
		}
	}
	t.blockReports(c, file, run.Reports, bag, t.under)
}

// hider is the evaluator, when it tells which frames a stack cut.
func (r *Runner) hider() verify.Hider {
	h, _ := r.ev.(verify.Hider)
	return h
}

// placed is the path of a false check's finding: at, else the value visited, then its `at` field.
func (t *traversal) placed(at *verify.Path, onField bool, c *syntax.CheckDecl) *verify.Path {
	p := at
	if p == nil {
		p = t.here()
	}
	if onField {
		p = p.Field(c.At.Name)
	}
	return p
}

// tell tells the evaluator of a false one-line check reported at path, when it listens.
func (r *Runner) tell(c *syntax.CheckDecl, self value.Value, path string, run Run) {
	if r.teller != nil {
		r.teller.Reported(c, self, path, run)
	}
}

// blockReports reports each fail and warn call at the provenance of its `at` value, under a precomputed result's frame if any.
func (r *Runner) blockReports(c *syntax.CheckDecl, file *syntax.File, reports []Report, bag *diag.Bag, under *diag.Frame) {
	for _, rep := range reports {
		var site verify.Site
		if rep.At != nil {
			site = verify.SiteOf(rep.At)
		}
		b := diag.E5002.At(site.Span, rep.Message)
		if rep.Warn {
			b = diag.W5002.At(site.Span, rep.Message)
		}
		if under != nil {
			site.ReportUnder(r.decorate(b, file, c), *under, bag, r.hider())
			continue
		}
		site.Report(r.decorate(b, file, c), r.pathOf(rep.At), bag)
	}
}

// oneLine is the finding of a false one-line check: E5001, or W5001 for warn.
func oneLine(c *syntax.CheckDecl, at source.Span, message string) *diag.Builder {
	if c.Keyword == syntax.KwWarn {
		return diag.W5001.At(at, message)
	}
	return diag.E5001.At(at, message)
}

// decorate names the check and relates its declaration (API.md F4).
func (r *Runner) decorate(b *diag.Builder, file *syntax.File, c *syntax.CheckDecl) *diag.Builder {
	name := checkName(c)
	if file != nil {
		b.Related(file.Span(c), diag.NoteCheck(name))
	}
	return b.Check(name)
}

func checkName(c *syntax.CheckDecl) string {
	if c.Name == nil {
		return ""
	}
	return c.Name.Name
}

// keyword is the span of a package check's `check` or `warn` keyword.
func keyword(file *syntax.File, c *syntax.CheckDecl) source.Span {
	if file == nil {
		return source.Span{}
	}
	for i := c.First(); i <= c.Last() && int(i) < len(file.Tokens); i++ {
		if tok := file.Tokens[i]; tok.Kind == c.Keyword {
			return source.Span{File: file.Src.ID, Start: tok.Start, End: tok.End}
		}
	}
	return file.Span(c)
}

// Placed is where a false check c of rec is reported: its `at` field's value (true), else rec (EVALUATION.md §8.3).
func Placed(c *syntax.CheckDecl, rec *value.Record) (value.Value, bool) {
	if c.At == nil {
		return rec, false
	}
	for i, f := range verify.Fields(rec.T) {
		if f.Name == c.At.Name && i < len(rec.Fields) && rec.Fields[i] != nil {
			return rec.Fields[i], true
		}
	}
	return rec, false
}

// reads is the fields a one-line check's condition reads, identifiers and self.f resolving to
// fields of its record or case, in first-use order (VIEWMODEL.md J15).
func (r *Runner) reads(c *syntax.CheckDecl, rec *value.Record) []string {
	if r.info == nil || c.Cond == nil {
		return nil
	}
	fields := verify.Fields(rec.T)
	var out []string
	syntax.Inspect(c.Cond, func(n syntax.Node) bool {
		name, ok := r.fieldRead(n)
		if ok && !slices.Contains(out, name) && slices.ContainsFunc(fields, func(f *types.Field) bool { return f.Name == name }) {
			out = append(out, name)
		}
		return true
	})
	return out
}

// fieldRead is the field an identifier or `self.f` resolves to.
func (r *Runner) fieldRead(n syntax.Node) (string, bool) {
	var obj check.Object
	switch n := n.(type) {
	case *syntax.IdentExpr:
		obj = r.info.Uses[n]
	case *syntax.SelectorExpr:
		if sel := r.info.Selections[n]; sel != nil && sel.Kind == check.SelField && isSelf(n.X) {
			obj = sel.Obj
		}
	}
	if obj == nil || obj.Kind() != check.ObjField {
		return "", false
	}
	return obj.Name(), true
}

func isSelf(e syntax.Expr) bool {
	_, ok := e.(*syntax.SelfExpr)
	return ok
}
