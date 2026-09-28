package rules

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// prop is a field property: what it is valid on, and how its value is checked.
type prop struct {
	on    []diag.Kind
	valid func(named) bool // a further condition on a field, nil for none
	check func(v *view, n named, fi *syntax.FieldItem)
}

// props are the properties of VIEWMODEL.md §3.5; another name is E1613.
var props map[string]prop

// rivals pairs `control` and `widget`, which one field cannot have both of (VIEWMODEL.md G15).
var rivals map[string]string

func init() {
	rivals = map[string]string{syntax.PropControl: syntax.PropWidget, syntax.PropWidget: syntax.PropControl}
	field, method := []diag.Kind{diag.KindField}, []diag.Kind{diag.KindField, diag.KindMethod}
	choice := []diag.Kind{diag.KindMember, diag.KindCase}
	plain := func(v *view, _ named, fi *syntax.FieldItem) {
		if s, ok := fi.Value.(syntax.StrLit); ok {
			v.plain(s)
		}
	}
	props = map[string]prop{
		syntax.PropHelp:        {on: slices.Concat(method, choice), check: plain},
		syntax.PropUnit:        {on: field, check: (*view).unitType},
		syntax.PropControl:     {on: field, check: (*view).control},
		syntax.PropWidget:      {on: field, check: (*view).widget},
		syntax.PropReadonly:    {on: field},
		syntax.PropHidden:      {on: method, check: (*view).hidden},
		syntax.PropPlaceholder: {on: field, check: plain},
		syntax.PropWhen:        {on: method},
		syntax.PropNone:        {on: field, valid: optionalField, check: plain},
		syntax.PropStep:        {on: field, valid: listField, check: func(v *view, _ named, fi *syntax.FieldItem) { v.step(fi.Value) }},
		syntax.PropIcon:        {on: choice, check: studioProp(syntax.StudioIcon, diag.KindIcon)},
		syntax.PropTone:        {on: choice, check: studioProp(syntax.StudioTone, diag.KindTone)},
	}
}

// props checks a member's properties against what it names (VIEWMODEL.md §3.5, G15).
func (v *view) props(f *syntax.ViewField, n named) {
	if f.Props == nil {
		return
	}
	for _, it := range f.Props.Items {
		if fi, ok := it.(*syntax.FieldItem); ok && fi.Name != nil && fi.Value != nil {
			v.prop(f, n, fi)
		}
	}
}

// propKey is a property of an item: on whichever of its lines it is set.
type propKey struct {
	item any
	name string
}

// givenAt is where one of keys already has the property name.
func (c *checker) givenAt(keys []any, name string) (source.Span, bool) {
	for _, k := range keys {
		if s, ok := c.given[propKey{k, name}]; ok {
			return s, true
		}
	}
	return source.Span{}, false
}

// prop is one property: E1613 given twice for the item, on one line or two, or where it does
// not apply; E1634 beside its rival.
func (v *view) prop(f *syntax.ViewField, n named, fi *syntax.FieldItem) {
	name, at := fi.Name.Name, v.span(fi.Name)
	if first, dup := v.c.givenAt(n.keys, name); dup {
		v.report(diag.E1613.AtTwice(at, name, first))
		return
	}
	for _, k := range n.keys {
		v.c.given[propKey{k, name}] = at
	}
	p, known := props[name]
	if !known || !slices.Contains(p.on, n.kind) || p.valid != nil && !p.valid(n) {
		v.report(diag.E1613.AtProperty(at, name, n.kind))
		return
	}
	if _, rival := v.c.givenAt(n.keys, rivals[name]); rival {
		v.report(diag.E1634.At(at, f.Name.Name))
	}
	if p.check != nil {
		p.check(v, n, fi)
	}
}

// optionalField is a field whose type is optional (`none`, VIEWMODEL.md §3.5).
func optionalField(n named) bool {
	return n.field != nil && n.field.Type.Base().Kind() == types.Optional
}

// listField is a field holding a list (`step`, VIEWMODEL.md §3.5, §7.7).
func listField(n named) bool {
	return n.field != nil && shape.ElemOf(shape.StripOptional(n.field.Type)) != nil
}

// hidden is `hidden: true` on a required field without a default: W1641 (VIEWMODEL.md L15).
func (v *view) hidden(n named, fi *syntax.FieldItem) {
	b, ok := fi.Value.(*syntax.BoolLit)
	f := n.field
	if !ok || !b.Value || f == nil || f.Default != nil || f.Input != nil || f.Type.Base().Kind() == types.Optional {
		return
	}
	v.report(diag.W1641.At(v.span(fi), f.Name))
}

// unitType is `unit` on a field of numbers (VIEWMODEL.md §3.5); CheckUnits checks its name.
func (v *view) unitType(n named, fi *syntax.FieldItem) {
	if !holdsNumbers(n.field.Type) {
		v.report(diag.E1611.At(v.span(fi.Value), n.field.Type))
	}
}

// holdsNumbers is a number, or a list or map of numbers, the optional stripped (§3.5).
func holdsNumbers(t types.Type) bool {
	t = shape.StripOptional(t)
	return numeric(t) || numeric(shape.ElemOf(t)) || numeric(shape.ValueOf(t))
}

// numeric is an integer type, Float or Float32.
func numeric(t types.Type) bool {
	return t != nil && shape.KindIn(t, types.Int, types.Float)
}
