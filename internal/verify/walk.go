package verify

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// walk verifies v against its declared type t, never through refs (EVALUATION.md §5).
func (w *walker) walk(v value.Value, t types.Type, at *Path, sc scope) {
	if v == nil || w.stopped() {
		return
	}
	if _, isNone := v.(*value.None); isNone {
		return
	}
	if base, ok := w.peel(v, t, at); ok {
		w.dispatch(v, base, at, sc)
	}
}

// peel checks each refinement layer of t on v and returns the type under them; false: the
// value needs no more checks here, or its dependent type is left to its milestone.
func (w *walker) peel(v value.Value, t types.Type, at *Path) (types.Type, bool) {
	for {
		switch x := t.(type) {
		case *types.Alias:
			t = x.Def
		case *types.Refined:
			w.refinement(v, x, at)
			t = x.Of
		case *types.OptionalType:
			t = x.Elem
		case *types.LitUnionType:
			if s, ok := v.(*value.Str); ok && slices.Contains(x.Literals, s.V) {
				return nil, false
			}
			t = x.Of
		case *types.TypeAppType, *types.DepUnionType:
			return nil, false
		default:
			return t, true
		}
	}
}

// dispatch checks what a value's kind implies, then walks its parts.
func (w *walker) dispatch(v value.Value, t types.Type, at *Path, sc scope) {
	switch x := v.(type) {
	case *value.Float:
		w.finite(x, basicOf(t, x.T), at)
	case *value.Member:
		w.member(x, at, sc)
	case *value.Record:
		w.record(x, at, sc)
	case *value.List:
		w.list(x, t, at, sc)
	case *value.Map:
		w.mapping(x, t, at, sc)
	case *value.Table:
		w.table(x, t, at, sc)
	case *value.Ref:
		w.ref(x, t, sc, at)
	case *value.Pair:
		w.pair(x, t, at, sc)
	}
}

func (w *walker) record(r *value.Record, at *Path, sc scope) {
	if c, ok := r.T.Base().(*types.CaseType); ok {
		w.retiredCase(r, c, at, sc)
	}
	for i, f := range Fields(r.T) {
		if i < len(r.Fields) {
			w.walk(r.Fields[i], f.Type, at.Field(f.Name), sc)
		}
	}
}

// list walks the elements; a keyed list's are paths by key, and its keys are unique.
func (w *walker) list(l *value.List, t types.Type, at *Path, sc scope) {
	lt := listOf(t, l.T)
	if lt == nil {
		return
	}
	for i, e := range l.Elems {
		p := at.Index(i)
		if r, ok := e.(*value.Record); ok && lt.KeyedBy != nil && r.Ident != nil {
			p = at.Key(r.Ident.Key)
		}
		w.walk(e, lt.Elem, p, sc)
	}
	if lt.KeyedBy != nil {
		w.uniqueKeys(l, lt, at)
	}
}

// mapping walks each key, then its value, in insertion order.
func (w *walker) mapping(m *value.Map, t types.Type, at *Path, sc scope) {
	var kt, vt types.Type
	switch mt := underOf(t, m.T).(type) {
	case *types.MapType:
		kt, vt = mt.Key, mt.Value
	case *types.DepMapType:
		vt = mt.Value
	}
	for i, k := range m.Keys {
		p := at.Key(KeyOf(k))
		w.walk(k, kt, p, sc)
		if i < len(m.Vals) {
			w.walk(m.Vals[i], vt, p, sc)
		}
	}
}

// table walks each entry in the scope of that entry, then checks its @stable values.
func (w *walker) table(tv *value.Table, t types.Type, at *Path, sc scope) {
	tt, ok := underOf(t, tv.T).(*types.TableType)
	if !ok {
		return
	}
	for _, e := range tv.Entries {
		if e == nil || e.Ident == nil {
			continue
		}
		p := at.Entry(e.Ident.Key)
		w.walk(e, tt.Elem, p, scope{entry: p.String(), retired: sc.retired || e.Ident.Retired})
	}
	if tt.Stable {
		w.uniqueStable(tv, tt, at)
	}
}

func (w *walker) pair(p *value.Pair, t types.Type, at *Path, sc scope) {
	var a, b types.Type
	if pt, ok := underOf(t, p.T).(*types.PairType); ok {
		a, b = pt.A, pt.B
	}
	w.walk(p.A, a, at, sc)
	w.walk(p.B, b, at, sc)
}

// Fields is the fields of a record, case or applied record type, in declaration order.
func Fields(t types.Type) []*types.Field {
	if t == nil {
		return nil
	}
	switch d := t.Base().(type) {
	case *types.RecordType:
		return d.Fields
	case *types.CaseType:
		return d.Fields
	case *types.AppliedRecord:
		return d.Rec.Fields
	}
	return nil
}

// underOf is the declared type t when known, else the type the value was stored as, with
// their alias and refinement layers removed.
func underOf(t, stored types.Type) types.Type {
	if t != nil {
		return t.Base()
	}
	if stored != nil {
		return stored.Base()
	}
	return nil
}

func listOf(t, stored types.Type) *types.ListType {
	lt, _ := underOf(t, stored).(*types.ListType)
	return lt
}

// basicOf is the scalar type a number was declared or stored as.
func basicOf(t, stored types.Type) types.Basic {
	b, _ := underOf(t, stored).(types.Basic)
	return b
}
