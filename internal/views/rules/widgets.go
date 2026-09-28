package rules

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// widget is `widget: w`: a widget the studio does not declare is E1610, one whose `value`
// parameter the field's type does not match E1608 (VIEWMODEL.md G16, G20).
func (v *view) widget(n named, fi *syntax.FieldItem) {
	id, ok := fi.Value.(*syntax.IdentExpr)
	s := v.c.studio
	if !ok || s == nil {
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
	t := n.field.Type
	if opt, isOpt := t.Base().(*types.OptionalType); !fits(t, p) && (!isOpt || !fits(opt.Elem, p)) {
		v.report(diag.E1608.At(v.span(id), id.Name, p, t, n.field.Name))
	}
}

// fits reports t matching the widget parameter type p: equal once aliases, refinements,
// `where` and `keyed by` are removed on both sides, `_` matching anything (VIEWMODEL.md G20).
func fits(t, p types.Type) bool {
	t, p = t.Base(), p.Base()
	if p.Kind() == types.Any {
		return true
	}
	switch x := p.(type) {
	case *types.OptionalType:
		y, ok := t.(*types.OptionalType)
		return ok && fits(y.Elem, x.Elem)
	case *types.ListType:
		y, ok := t.(*types.ListType)
		return ok && fits(y.Elem, x.Elem)
	case *types.MapType:
		y, ok := t.(*types.MapType)
		return ok && fits(y.Key, x.Key) && fits(y.Value, x.Value)
	case *types.TableType:
		y, ok := t.(*types.TableType)
		return ok && y.Stable == x.Stable && fits(y.Elem, x.Elem)
	}
	return types.Identical(t, p)
}

// widgetDecls checks p's widget declarations: `siblings` is `[P]` (E1631); a default widget's
// `value` is a named record, variant or enum, optionally in a list (E1630), one per type (E1629)
// (VIEWMODEL.md G21, G22).
func (c *checker) widgetDecls(p *check.Package, bag *diag.Bag) {
	defaults := map[defaultKey]check.Object{}
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
		key, ok := defaultOf(o.Type())
		switch first := defaults[key]; {
		case !ok:
			diag.E1630.At(o.File().Span(d.Params[0].Type), o.Type()).Report(bag)
		case first != nil:
			diag.E1629.At(o.File().Span(d.Name), o.Type().String(), first.Name(), o.Name()).Report(bag)
		default:
			defaults[key] = o
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

// defaultKey is what a default widget's `value` type is: a named type, or a list of one.
type defaultKey struct {
	named types.Type
	list  bool
}

// defaultOf is the key of a default widget's `value` type: a named record, variant or enum,
// optionally in a list; false for any other type (E1630).
func defaultOf(p types.Type) (defaultKey, bool) {
	k := defaultKey{named: p.Base()}
	if e := shape.ElemOf(k.named); e != nil {
		k = defaultKey{named: e.Base(), list: true}
	}
	switch k.named.(type) {
	case *types.RecordType, *types.VariantType, *types.EnumType:
		return k, true
	}
	return defaultKey{}, false
}
