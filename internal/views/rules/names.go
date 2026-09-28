package rules

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// named is what a member item, a column or a filter names (VIEWMODEL.md §3.3).
type named struct {
	kind  diag.Kind // KindField, KindMethod, KindCase or KindMember
	field *types.Field
	keys  []any // what its label and properties belong to: every field the name matches (L18), else its object
}

// caseField is a field of a case of an inline variant, and its case.
type caseField struct {
	ct    *types.CaseType
	field *types.Field
}

// member is a field item, in group (nil at view level): its name, label and properties (§3.3).
func (v *view) member(f *syntax.ViewField, group *syntax.Ident) {
	if f.Name == nil {
		return
	}
	n, ok := v.resolve(f.Name)
	if !ok {
		return
	}
	if group == nil || v.place(f.Name, n, group) {
		v.label(f, n)
	} else {
		v.plain(f.Label)
	}
	v.props(f, n)
}

// resolve is what a name names: E1602 where check found nothing (G8), E1616 for a method with
// parameters, E1628 for case fields of different types.
func (v *view) resolve(id *syntax.Ident) (named, bool) {
	o := v.c.info.NameUses[id]
	if o == nil {
		v.report(diag.E1602.At(v.span(id), v.name, id.Name))
		return named{}, false
	}
	switch o.Kind() {
	case check.ObjField:
		fs := v.fieldsNamed(id)
		if len(fs) == 0 {
			return named{}, false
		}
		n := named{kind: diag.KindField, field: fs[0]}
		for _, f := range fs {
			n.keys = append(n.keys, f)
		}
		return n, true
	case check.ObjMethod:
		if m := methodNamed(v.methods, id.Name); m != nil && len(m.Type.Params) > 0 {
			v.report(diag.E1616.At(v.span(id), id.Name))
			return named{}, false
		}
		return named{kind: diag.KindMethod, keys: []any{o}}, true
	case check.ObjCase:
		return named{kind: diag.KindCase, keys: []any{o}}, true
	case check.ObjMember:
		return named{kind: diag.KindMember, keys: []any{o}}, true
	default:
		return named{}, false
	}
}

// field is the target's field named id, else its first case field; nil after E1628 (G8).
func (v *view) field(id *syntax.Ident) *types.Field {
	if fs := v.fieldsNamed(id); len(fs) > 0 {
		return fs[0]
	}
	return nil
}

// fieldsNamed is the target's field named id, else every case field of the name, depth first
// (L18); none after E1628 (G8).
func (v *view) fieldsNamed(id *syntax.Ident) []*types.Field {
	if f := fieldNamed(v.fields, id.Name); f != nil {
		return []*types.Field{f}
	}
	var found []caseField
	seen := map[*types.VariantType]bool{}
	if v.variant != nil {
		casesFields(v.variant, id.Name, seen, &found)
	} else {
		inlineFields(v.fields, id.Name, seen, &found)
	}
	out := make([]*types.Field, len(found))
	for i, cf := range found {
		if !shape.SameType(cf.field.Type, found[0].field.Type) {
			v.report(diag.E1628.At(v.span(id), id.Name, v.caseNames(found)))
			return nil
		}
		out[i] = cf.field
	}
	return out
}

// caseNames names each case of found as `V.c`, qualified outside the view's package.
func (v *view) caseNames(found []caseField) []string {
	out := make([]string, len(found))
	for i, cf := range found {
		out[i] = v.qualified(cf.ct.Variant.Pkg, cf.ct.Variant.Name+dot+cf.ct.Name)
	}
	return out
}

// qualified is name, prefixed by pkg when it is not the view's package (ERRORS.md §1.3).
func (v *view) qualified(pkg, name string) string {
	if pkg == v.pkg || pkg == "" {
		return name
	}
	return pkg + dot + name
}

// inlineFields adds the fields named name of the cases of each `@json(inline)` variant field.
func inlineFields(fields []*types.Field, name string, seen map[*types.VariantType]bool, out *[]caseField) {
	for _, f := range fields {
		if vt, ok := f.Type.Base().(*types.VariantType); ok && f.Inline {
			casesFields(vt, name, seen, out)
		}
	}
}

// casesFields adds the fields named name of vt's cases in case order, nested inline variants
// depth first; seen stops a variant inlining itself.
func casesFields(vt *types.VariantType, name string, seen map[*types.VariantType]bool, out *[]caseField) {
	if seen[vt] {
		return
	}
	seen[vt] = true
	for _, ct := range vt.Cases {
		if f := fieldNamed(ct.Fields, name); f != nil {
			*out = append(*out, caseField{ct: ct, field: f})
		}
		inlineFields(ct.Fields, name, seen, out)
	}
}

func fieldNamed(fields []*types.Field, name string) *types.Field {
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func methodNamed(methods []*types.Method, name string) *types.Method {
	for _, m := range methods {
		if m.Name == name {
			return m
		}
	}
	return nil
}

// place records a name placed in a group: E1605 when it already is (G9), W1642 for a
// deprecated field (L6); false after E1605.
func (v *view) place(id *syntax.Ident, n named, group *syntax.Ident) bool {
	if first, dup := v.placed[id.Name]; dup {
		v.report(diag.E1605.At(v.span(id), id.Name, v.name, first))
		return false
	}
	v.placed[id.Name] = v.span(id)
	if n.field != nil && n.field.Deprecated != nil {
		v.report(diag.W1642.At(v.span(id), id.Name, group.Name))
	}
	return true
}

// label is a member's plain label; an item labelled twice, in one view or two, is E1614 (G10).
func (v *view) label(f *syntax.ViewField, n named) {
	v.plain(f.Label)
	if f.Label == nil {
		return
	}
	if first, dup := firstOf(v.c.labels, n.keys); dup {
		v.report(diag.E1614.At(v.span(f.Label), f.Name.Name, first))
		return
	}
	for _, k := range n.keys {
		v.c.labels[k] = v.span(f.Label)
	}
}

// firstOf is the first span seen records for one of keys.
func firstOf(seen map[any]source.Span, keys []any) (source.Span, bool) {
	for _, k := range keys {
		if s, ok := seen[k]; ok {
			return s, true
		}
	}
	return source.Span{}, false
}
