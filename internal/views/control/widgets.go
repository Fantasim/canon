package control

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// defaultWidget is a studio widget declared `default` (VIEWMODEL.md G22): its name, its `value`
// type and whether it takes `siblings` (G21).
type defaultWidget struct {
	name     string
	value    types.Type
	siblings bool
}

// defaultWidgets are the default widgets of the studio package p, in declaration order; a
// second one for a type (E1629) or one on another type (E1630) never applies.
func defaultWidgets(info *check.Info, p *check.Package) []defaultWidget {
	var out []defaultWidget
	for _, o := range p.Decls {
		d, ok := o.Decl().(*syntax.WidgetDecl)
		if !ok || o.Kind() != check.ObjWidget || info.Broken[o] || !d.Default.Valid() || len(d.Params) == 0 {
			continue
		}
		if o.Type() == nil || o.Type().Kind() == types.Error || !Defaultable(o.Type()) || taken(out, o.Type()) {
			continue
		}
		out = append(out, defaultWidget{name: o.Name(), value: o.Type(), siblings: len(d.Params) > 1})
	}
	return out
}

// Defaultable reports a type a default widget may take: a named record, variant or enum,
// optionally in a list (VIEWMODEL.md G22, E1630).
func Defaultable(p types.Type) bool {
	b := p.Base()
	if l, ok := b.(*types.ListType); ok {
		b = l.Elem.Base()
	}
	switch b.(type) {
	case *types.RecordType, *types.VariantType, *types.EnumType:
		return true
	}
	return false
}

// SameWidgetType reports two widget parameter types equal by G20: two default widgets on one
// type are E1629.
func SameWidgetType(a, b types.Type) bool { return fits(a, b) && fits(b, a) }

// taken reports a default widget already declared for p's type (E1629).
func taken(ws []defaultWidget, p types.Type) bool {
	for _, w := range ws {
		if SameWidgetType(p, w.value) {
			return true
		}
	}
	return false
}

// widget is the widget control of a field of type t (C1 steps 1 and 3, C41): its `widget`
// property when the studio declares it and t matches it (E1610, E1608), else, without a
// `control` or `widget` property, the first default widget t matches.
func (x *Index) widget(t types.Type, p Props) (vm.Control, bool) {
	if id, ok := shape.Unparen(p[syntax.PropWidget].value).(*syntax.IdentExpr); ok {
		o := x.info.Uses[id]
		if o == nil || o.Kind() != check.ObjWidget || x.info.Broken[o] || o.Type() == nil || !WidgetMatches(t, o.Type()) {
			return vm.Control{}, false
		}
		d, _ := o.Decl().(*syntax.WidgetDecl)
		return vm.Control{Kind: ctlWidget, Widget: o.Name(), Siblings: d != nil && len(d.Params) > 1}, true
	}
	if _, hinted := p[syntax.PropControl]; hinted {
		return vm.Control{}, false
	}
	if _, given := p[syntax.PropWidget]; given {
		return vm.Control{}, false
	}
	for _, w := range x.defaults {
		if WidgetMatches(t, w.value) {
			return vm.Control{Kind: ctlWidget, Widget: w.name, Siblings: w.siblings}, true
		}
	}
	return vm.Control{}, false
}

// WidgetMatches reports a field type t matching the widget parameter type p: t fits p, or is
// `T?` with T fitting it (VIEWMODEL.md G20, E1608).
func WidgetMatches(t, p types.Type) bool {
	if fits(t, p) {
		return true
	}
	o, ok := t.Base().(*types.OptionalType)
	return ok && fits(o.Elem, p)
}

// fits reports t equal to p once aliases, refinements, `where` and `keyed by` are removed on
// both sides, `_` in p matching anything at any depth (G20).
func fits(t, p types.Type) bool {
	t, p = t.Base(), p.Base()
	if p.Kind() == types.Any {
		return true
	}
	if t.Kind() != p.Kind() {
		return false
	}
	tp, pp := components(t), components(p)
	if tp == nil {
		return types.Identical(t, p)
	}
	for i := range tp {
		if !fits(tp[i], pp[i]) {
			return false
		}
	}
	return stable(t) == stable(p)
}

// components are what a composite of G20 compares; nil for any other type.
func components(t types.Type) []types.Type {
	switch x := t.(type) {
	case *types.OptionalType:
		return []types.Type{x.Elem}
	case *types.ListType:
		return []types.Type{x.Elem}
	case *types.TableType:
		return []types.Type{x.Elem}
	case *types.MapType:
		return []types.Type{x.Key, x.Value}
	}
	return nil
}

// stable reports a stable table.
func stable(t types.Type) bool {
	tt, ok := t.(*types.TableType)
	return ok && tt.Stable
}
