package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// decodedSites are what a loader reads of a class: each field's type (a pairs field's pair record fields, read slot by slot) and each stored fn's result.
func (s *stage) decodedSites(fields []*Field, fns []*ExportFn) []typeSite {
	var out []typeSite
	for _, f := range fields {
		if f.Input != nil || f.Optional && f.Type.Kind == types.Never {
			continue
		}
		out = append(out, readFieldSites(f, s.itemSpan(f, source.Span{}))...)
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			out = append(out, typeSite{t: &fn.Result, span: s.itemSpan(fn, source.Span{})})
		}
	}
	return out
}

// readFieldSites are what a loader reads of field f: its type, or the fields of its pair record, read slot by slot.
func readFieldSites(f *Field, span source.Span) []typeSite {
	if f.Pairs == nil || f.Type.Elem == nil {
		return []typeSite{{t: &f.Type, span: span}}
	}
	rec, ok := f.Type.Elem.Named.(*Record)
	if !ok {
		return nil
	}
	out := make([]typeSite, len(rec.Fields))
	for i, pf := range rec.Fields {
		out[i] = typeSite{t: &pf.Type, span: span}
	}
	return out
}

// checkDecodedType is E8019 `MapField` for a map a loader cannot read: gen/go's and gen/cpp's data loaders read maps, keys as WIRE.md §5.8 in file order (CODEGEN.md §5.9, DECISIONS 312); a map that holds a dependent key or value is DependentType's.
func (s *stage) checkDecodedType(u *unit, es *emitSite, site typeSite) {
	if typeHolds(site.t, unreadMap(es.e)) {
		u.reportGenConstruct(es, site.span, diag.KindMapField)
	}
}

// unreadMap accepts a map e's loader cannot read: any in a types-mode decoder (nlohmann::json keeps no key order, CODEGEN.md §5.13), or in a data-mode loader one whose key is none WIRE.md §5.8 gives a wire key; a dependent key or value is DependentType's (unreadDependent).
func unreadMap(e *Emit) func(*TypeRef) bool {
	return func(t *TypeRef) bool {
		return isMap(t) && (e.Mode != ModeData || t.Kind == types.Map && !readableKey(t.Key) && !isApp(t.Key))
	}
}

// readableKey is a map key a data loader reads: its wire key is the text WIRE.md §5.8 gives, a String, an integer, an enum, a ref or a literal union.
func readableKey(k *TypeRef) bool {
	return k != nil && (k.Kind == types.String || k.Kind == types.Int || k.Kind == types.Enum || k.Kind == types.Ref || k.Kind == types.LitUnion)
}

// isMap is a map type, a dependent map included (CODEGEN.md §4.2).
func isMap(t *TypeRef) bool { return t.Kind == types.Map || t.Kind == types.DepMap }

// decodedHolds reports what a loader reads of t, t itself or its elements through lists and optionals, that bad accepts; a record or variant is its own decoder's.
func decodedHolds(t *TypeRef, bad func(*TypeRef) bool) bool {
	for ; t != nil; t = t.Elem {
		if bad(t) {
			return true
		}
		if t.Kind != types.List && t.Kind != types.Optional {
			return false
		}
	}
	return false
}

// NumericMapKey reports a map key written as a canonical decimal integer (`0|-?[1-9][0-9]*`, WIRE.md §5.1, §5.8): an integer, an enum with @json(codes), a ref keyed by one; both data loaders read such a key by that rule.
func NumericMapKey(t TypeRef) bool {
	switch t.Kind {
	case types.Int:
		return true
	case types.Enum:
		e, ok := t.Named.(*Enum)
		return ok && e.JSONCodes && e.Codes != nil
	case types.Ref:
		return t.Key != nil && NumericMapKey(*t.Key)
	default:
		return false
	}
}

// StringWire reports a type written as a JSON string (WIRE.md §5.8): String, an enum without @json(codes), a ref keyed by one, a dependent type whose branches all are.
func StringWire(t *TypeRef) bool {
	switch t.Kind {
	case types.String:
		return true
	case types.Enum:
		e, ok := t.Named.(*Enum)
		return ok && !e.JSONCodes
	case types.Ref:
		return t.Key != nil && StringWire(t.Key)
	case types.TypeApp:
		d, ok := t.Named.(*Dependent)
		return ok && branchesStringWire(d)
	default:
		return false
	}
}

func branchesStringWire(d *Dependent) bool {
	for _, b := range d.Branches {
		if !StringWire(&b.Type) {
			return false
		}
	}
	return true
}
