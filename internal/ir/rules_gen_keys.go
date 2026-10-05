package ir

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// objectShape is how a loader lists a class's object keys (WIRE.md §5.5): extras beyond fields and fns; deep lists inline case keys (gen/cpp).
type objectShape struct {
	extras map[any][]string
	deep   bool
}

// objectExtras are a case's variant tag and a table record's `$id` and `$retired`, the keys beyond fields and fns (WIRE.md §5.11).
func (s *stage) objectExtras(u *unit, tables []*valueSite) map[any][]string {
	out := map[any][]string{}
	for _, t := range u.p.Types {
		if v, ok := t.(*Variant); ok {
			for _, c := range v.Cases {
				out[c] = []string{v.Tag}
			}
		}
	}
	for _, v := range tables {
		if rec, ok := tableElem(v.v.Type); ok {
			out[rec] = []string{fpDollar + GoIDStore, fpDollar + GoRetiredStore}
		}
	}
	return out
}

// tableElem is the record of a table type.
func tableElem(t TypeRef) (*Record, bool) {
	if t.Kind != types.Table || t.Elem == nil {
		return nil, false
	}
	rec, ok := t.Elem.Named.(*Record)
	return rec, ok
}

// checkInlineFolds is E8019 `InlineFoldedKey` at each field inlineFolds finds.
func (s *stage) checkInlineFolds(u *unit, es *emitSite, class any, shape objectShape) {
	for _, f := range inlineFolds(class, shape) {
		u.reportGenConstruct(es, s.itemSpan(f, source.Span{}), diag.KindInlineFoldedKey)
	}
}

// inlineFolds are the inline variant fields of class with a key equal to another key of its parent but for ASCII case (WIRE.md §5.6).
func inlineFolds(class any, shape objectShape) []*Field {
	fields, fns := classBody(class)
	all := append(slices.Clone(shape.extras[class]), shape.keys(fields, fns)...)
	var out []*Field
	for _, f := range fields {
		v, ok := f.Type.Named.(*Variant)
		if f.Inline && ok && foldsOnto(shape.inlineKeys(v), all) {
			out = append(out, f)
		}
	}
	return out
}

// keys are the object keys of fields and fns: first path segments, pairs slots, inline keys, then `$<fn>` (WIRE.md §5.11).
func (sh objectShape) keys(fields []*Field, fns []*ExportFn) []string {
	var out []string
	for _, f := range fields {
		out = append(out, sh.fieldKeys(f)...)
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			out = append(out, fpDollar+fn.Name)
		}
	}
	return out
}

func (sh objectShape) fieldKeys(f *Field) []string {
	v, isVariant := f.Type.Named.(*Variant)
	switch {
	case f.Input != nil, f.Optional && f.Type.Kind == types.Never:
		return nil
	case f.Pairs != nil:
		return pairKeys(f.Pairs)
	case f.Inline && isVariant && sh.deep:
		return sh.inlineKeys(v)
	case f.Inline && isVariant:
		return []string{v.Tag}
	case len(f.WirePath) > 0:
		return f.WirePath[:1]
	}
	return nil
}

// inlineKeys are an inline variant's keys in its parent's object: its tag, then every key of its cases with fields.
func (sh objectShape) inlineKeys(v *Variant) []string {
	out := []string{v.Tag}
	for _, c := range v.Cases {
		if len(c.Fields) > 0 {
			out = append(out, sh.keys(c.Fields, c.Methods)...)
		}
	}
	return out
}

// pairKeys are a pairs field's slot keys, each template's `{i}` replaced by 0 to Slots-1 (WIRE.md §5.14).
func pairKeys(p *types.Pairs) []string {
	var out []string
	for _, tmpl := range p.Keys {
		for i := range p.Slots {
			before, after, _ := strings.Cut(tmpl, pairsOpen)
			_, rest, _ := strings.Cut(after, pairsClose)
			out = append(out, before+strconv.Itoa(i)+rest)
		}
	}
	return out
}

// foldsOnto reports a key of own equal, but for ASCII letter case, to a key of all own does not hold.
func foldsOnto(own, all []string) bool {
	return slices.ContainsFunc(own, func(k string) bool {
		return slices.ContainsFunc(all, func(p string) bool { return !slices.Contains(own, p) && asciiFold(k, p) })
	})
}

// asciiFold reports a and b equal but for ASCII letter case.
func asciiFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		if lowerASCII(a[i]) != lowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}
