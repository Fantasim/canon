package verify

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// uniqueKeys reports each keyed-list key already used, at the second key (TYPES.md §9.1).
func (w *walker) uniqueKeys(l *value.List, lt *types.ListType, at *Path) {
	key := lt.KeyedBy
	first := map[value.Key]value.Value{}
	for _, e := range l.Elems {
		r, ok := e.(*value.Record)
		if !ok || r.Ident == nil || key.Index >= len(r.Fields) || r.Fields[key.Index] == nil {
			continue
		}
		k := r.Fields[key.Index]
		prev, dup := first[r.Ident.Key]
		if !dup {
			first[r.Ident.Key] = k
			continue
		}
		s := SiteOf(k)
		b := w.src.related(diag.E3102.AtKey(s.Span, k, SiteOf(prev).Span), lt)
		w.flag(s, b, k, at.Key(r.Ident.Key).Field(key.Name))
	}
}

// uniqueResolved reports a resolved key equal to an earlier one, `one` then `"one"` (E3322, TYPES.md §5.2).
func (w *walker) uniqueResolved(m *value.Map, at *Path) {
	seen := &value.Map{}
	for _, k := range m.Keys {
		if i, _ := seen.Lookup(k, sameKey); i >= 0 {
			s := SiteOf(k)
			w.report(s, diag.E3322.At(s.Span, k), at)
			w.invalid(m)
			continue
		}
		seen.Keys = append(seen.Keys, k)
	}
}

func sameKey(a, b value.Value) (bool, bool) { return value.Equal(a, b), true }

// stableValue is a @stable value as the lock compares it: an integer or a string.
type stableValue struct {
	isString bool
	i        int64
	s        string
}

// uniqueStable reports a @stable value held twice, retired entries included (LOCK.md §1).
func (w *walker) uniqueStable(tv *value.Table, tt *types.TableType, at *Path) {
	for _, f := range Fields(tt.Elem) {
		if !f.Stable {
			continue
		}
		first := map[stableValue]value.Value{}
		for _, e := range tv.Entries {
			if e == nil || e.Ident == nil || f.Index >= len(e.Fields) {
				continue
			}
			v := e.Fields[f.Index]
			sv, ok := stableOf(v)
			prev, dup := first[sv]
			switch {
			case !ok:
			case !dup:
				first[sv] = v
			default:
				s := SiteOf(v)
				b := w.src.related(diag.E3102.AtStable(s.Span, v, SiteOf(prev).Span), tt)
				w.flag(s, b, v, at.Entry(e.Ident.Key).Field(f.Name))
			}
		}
	}
}

func stableOf(v value.Value) (stableValue, bool) {
	switch x := v.(type) {
	case *value.Int:
		return stableValue{i: x.V}, true
	case *value.Str:
		return stableValue{isString: true, s: x.V}, true
	}
	return stableValue{}, false
}

// Codes reports a @codes code held twice, retired members included (TYPES.md §8.1).
func (v *Verifier) Codes(enum check.Object) (bool, error) {
	e, ok := enum.Type().(*types.EnumType)
	if !ok || e.Codes == nil {
		return true, nil
	}
	bag := v.bags[enum.Pkg()]
	if bag == nil {
		return false, fmt.Errorf(fmtNoBag, ErrNoBag, enum.Pkg())
	}
	spans, codes := codeSpans(enum.File(), e)
	first := map[int64]int{}
	valid := true
	for i, m := range e.Members {
		j, dup := first[m.Code]
		switch {
		case !m.HasCode:
		case !dup:
			first[m.Code] = i
		default:
			b := diag.E3102.AtCode(spans[i], m.Code, spans[j])
			if codes.File != source.NoFile {
				b.Related(codes, diag.NoteSource(codes))
			}
			b.Report(bag)
			valid = false
		}
	}
	return valid, nil
}

// codeSpans locates each member's code (or the member without one) and the @codes annotation.
func codeSpans(file *syntax.File, e *types.EnumType) ([]source.Span, source.Span) {
	out := make([]source.Span, len(e.Members))
	if e.Decl == nil || file == nil {
		return out, source.Span{}
	}
	for i, m := range e.Members {
		node, ok := memberNode(e, m.Name).(*syntax.EnumMember)
		switch {
		case !ok:
		case node.Value != nil:
			out[i] = file.Span(node.Value)
		default:
			out[i] = file.Span(node)
		}
	}
	for _, a := range e.Decl.Annotations {
		if a.Name != nil && a.Name.Name == codesAnnotation {
			return out, file.Span(a)
		}
	}
	return out, source.Span{}
}
