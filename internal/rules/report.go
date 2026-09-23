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
		run := r.ev.Run(ctx, c, nil)
		if run.Aborted {
			continue
		}
		file := obj.File()
		if run.Failed {
			at := keyword(file, c)
			r.decorate(oneLine(c, at, run.Message), file, c).Report(bag)
		}
		r.blockReports(c, file, run.Reports, bag)
	}
	return nil
}

// report places the findings of one run of an instance check (EVALUATION.md §8.3).
func (t *traversal) report(c *syntax.CheckDecl, run Run, rec *value.Record, at *verify.Path) {
	if run.Aborted {
		return
	}
	bag, file := t.bag, t.files[c]
	if run.Failed {
		v, p := value.Value(rec), at
		if f, ok := field(rec, c.At); ok {
			v, p = f, at.Field(c.At.Name)
		}
		site := verify.SiteOf(v)
		b := t.decorate(oneLine(c, site.Span, run.Message), file, c)
		if c.At == nil {
			b.Reads(t.reads(c, rec))
		}
		site.Report(b, p, bag)
	}
	t.blockReports(c, file, run.Reports, bag)
}

// blockReports reports each fail and warn call at the provenance of its `at` value.
func (r *Runner) blockReports(c *syntax.CheckDecl, file *syntax.File, reports []Report, bag *diag.Bag) {
	for _, rep := range reports {
		var site verify.Site
		if rep.At != nil {
			site = verify.SiteOf(rep.At)
		}
		b := diag.E5002.At(site.Span, rep.Message)
		if rep.Warn {
			b = diag.W5002.At(site.Span, rep.Message)
		}
		site.Report(r.decorate(b, file, c), r.paths[rep.At], bag)
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

// field is the value of the field `at` names in an instance; false without `at` or that value.
func field(rec *value.Record, at *syntax.Ident) (value.Value, bool) {
	if at == nil {
		return nil, false
	}
	for i, f := range verify.Fields(rec.T) {
		if f.Name == at.Name && i < len(rec.Fields) && rec.Fields[i] != nil {
			return rec.Fields[i], true
		}
	}
	return nil, false
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
