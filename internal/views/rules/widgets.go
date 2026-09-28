package rules

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// widget is `widget: w`: a widget the studio does not declare is E1610, a string or an
// expression too (log-2026-09-28 check follow-ups 2); one whose `value` parameter the field's
// type does not match is E1608 (VIEWMODEL.md G16, G20).
func (v *view) widget(n named, fi *syntax.FieldItem) {
	s := v.c.studio
	if s == nil {
		return
	}
	id, ok := shape.Unparen(fi.Value).(*syntax.IdentExpr)
	if !ok {
		v.report(diag.E1610.At(v.span(fi.Value), diag.KindWidget, encode.SourceText(v.file, fi.Value), s.path))
		return
	}
	o := v.c.info.Uses[id]
	if o == nil || o.Kind() != check.ObjWidget {
		v.report(diag.E1610.At(v.span(id), diag.KindWidget, id.Name, s.path))
		return
	}
	p := o.Type()
	if s.broken[o] || isError(p) {
		return
	}
	if t := n.field.Type; !control.WidgetMatches(t, p) {
		v.report(diag.E1608.At(v.span(id), id.Name, p, t, n.field.Name))
	}
}

// widgetDecls checks p's widget declarations: `siblings` is `[P]` (E1631); a default widget's
// `value` is a named record, variant or enum, optionally in a list (E1630), one per type (E1629)
// (VIEWMODEL.md G21, G22).
func (c *checker) widgetDecls(p *check.Package, bag *diag.Bag) {
	var defaults []check.Object
	for _, o := range p.Decls {
		d, ok := o.Decl().(*syntax.WidgetDecl)
		if !ok || o.Kind() != check.ObjWidget || c.info.Broken[o] || isError(o.Type()) || len(d.Params) == 0 {
			continue
		}
		if len(d.Params) > 1 {
			c.siblings(o, d.Params[1], bag)
		}
		if !d.Default.Valid() {
			continue
		}
		first := slices.IndexFunc(defaults, func(f check.Object) bool { return control.SameWidgetType(f.Type(), o.Type()) })
		switch {
		case !control.Defaultable(o.Type()):
			diag.E1630.At(o.File().Span(d.Params[0].Type), o.Type()).Report(bag)
		case first >= 0:
			diag.E1629.At(o.File().Span(d.Name), o.Type().String(), defaults[first].Name(), o.Name()).Report(bag)
		default:
			defaults = append(defaults, o)
		}
	}
}

// siblings is E1631 unless the `siblings` parameter is exactly `[P]`, P the `value` type (G21).
func (c *checker) siblings(o check.Object, sib *syntax.Param, bag *diag.Bag) {
	t := c.info.TypeExprs[sib.Type]
	if t == nil || isError(t) {
		return
	}
	if l, ok := t.Base().(*types.ListType); ok && l.KeyedBy == nil && shape.SameType(l.Elem, o.Type()) {
		return
	}
	diag.E1631.At(o.File().Span(sib.Type), o.Type(), t).Report(bag)
}
