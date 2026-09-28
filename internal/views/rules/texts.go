package rules

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// plain is a plain text: its first unescaped `{` is E1615 (VIEWMODEL.md G14).
func (v *view) plain(s syntax.StrLit) {
	sl, ok := s.(*syntax.StringLit)
	if !ok {
		return
	}
	for _, p := range sl.Parts {
		if p.Interp != nil {
			v.report(diag.E1615.At(v.span(p.Interp)))
			return
		}
	}
}

// template is a view template: the first use of a field named `key` or `index` is W1604
// (VIEWMODEL.md G11).
func (v *view) template(s syntax.StrLit) {
	sl, ok := s.(*syntax.StringLit)
	if !ok {
		return
	}
	for _, p := range sl.Parts {
		if p.Interp != nil {
			syntax.Inspect(p.Interp, v.hiding)
		}
	}
}

// hiding reports a name reading a field that hides the magic name of the same spelling, once
// per view and name.
func (v *view) hiding(n syntax.Node) bool {
	id, ok := n.(*syntax.IdentExpr)
	if !ok || id.Name != magicKey && id.Name != magicIndex || v.hides[id.Name] {
		return true
	}
	if o := v.c.info.Uses[id]; o != nil && o.Kind() == check.ObjField && fieldNamed(v.fields, id.Name) != nil {
		v.hides[id.Name] = true
		v.report(diag.W1604.At(v.span(id), id.Name, v.name))
	}
	return true
}

// step is a `step` text: a bare `{index}` only (VIEWMODEL.md §3.5).
func (v *view) step(e syntax.Expr) {
	sl, ok := e.(*syntax.StringLit)
	if !ok {
		return
	}
	for _, p := range sl.Parts {
		if p.Interp == nil {
			continue
		}
		if id, isName := p.Interp.X.(*syntax.IdentExpr); isName && id.Name == magicIndex && p.Interp.Spec == nil {
			continue
		}
		at := v.span(p.Interp)
		v.report(diag.E1623.At(at, string(v.file.Src.Content[at.Start:at.End])))
	}
}
