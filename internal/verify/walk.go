package verify

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// walk verifies v against its declared type t, never through refs, converting its dependent values (EVALUATION.md §5).
func (w *walker) walk(v value.Value, t types.Type, at *Path, sc scope) value.Value {
	switch {
	case v == nil || w.stopped():
		return v
	case sc.dep != nil:
		return w.judge(v, t, at, sc) // a part of a list, map or pair a dependent type computed
	case isNone(v):
		return w.walkNone(v, t, at, sc)
	}
	base, optional, ok := layers(v, t, func(r *types.Refined) { w.refinement(v, r, at) })
	if !ok {
		return v
	}
	if app, isApp := base.(*types.TypeAppType); isApp {
		return w.dependent(v, app, optional, at, sc)
	}
	return w.dispatch(v, base, at, sc)
}

// walkNone judges none given to a dependent type (TYPES.md §11.6); none needs nothing else.
func (w *walker) walkNone(v value.Value, t types.Type, at *Path, sc scope) value.Value {
	if base, optional, ok := layers(v, t, nil); ok && base != nil && base.Kind() == types.TypeApp {
		return w.dependent(v, base.(*types.TypeAppType), optional, at, sc)
	}
	return v
}

// layers removes t's aliases, refinements (each passed to refined, if any), optionals and
// literal unions over v, and returns the type under them and whether an optional was crossed;
// false: v needs no more checks, a literal of the union or a static view.
func layers(v value.Value, t types.Type, refined func(*types.Refined)) (types.Type, bool, bool) {
	optional := false
	for {
		switch x := t.(type) {
		case *types.Alias:
			t = x.Def
		case *types.Refined:
			if refined != nil {
				refined(x)
			}
			t = x.Of
		case *types.OptionalType:
			t, optional = x.Elem, true
		case *types.LitUnionType:
			if s, ok := v.(*value.Str); ok && slices.Contains(x.Literals, s.V) {
				return nil, optional, false
			}
			t = x.Of
		case *types.DepUnionType:
			return nil, optional, false
		default:
			return t, optional, true
		}
	}
}

// dispatch checks what a value's kind implies, then walks its parts.
func (w *walker) dispatch(v value.Value, t types.Type, at *Path, sc scope) value.Value {
	switch x := v.(type) {
	case *value.Float:
		w.finite(x, basicOf(t, x.T), at)
	case *value.Member:
		w.member(x, at, sc)
	case *value.Record:
		return w.record(x, t, at, sc)
	case *value.List:
		return w.list(x, t, at, sc)
	case *value.Map:
		return w.mapping(x, t, at, sc)
	case *value.Table:
		return w.table(x, t, at, sc)
	case *value.Ref:
		w.ref(x, t, sc, at)
	case *value.Pair:
		return w.pair(x, t, at, sc)
	}
	return v
}

// record verifies an instance once for every root reaching it, sharing its copy (EVALUATION.md §4.2).
func (w *walker) record(r *value.Record, t types.Type, at *Path, sc scope) value.Value {
	if w.stage == nil {
		return w.fields(r, t, at, sc)
	}
	w.appliedArgs(r, t, at, sc)
	if m, ok := w.stage.Verified(r); ok {
		w.voidRec()
		return w.replay(m.(*memo), t, at, sc) // the seam keeps verify's memo opaque; only remembered puts one there
	}
	return w.remembered(r, t, at, sc)
}

// fields walks each field in the record's env; a field converted makes a copy.
func (w *walker) fields(r *value.Record, t types.Type, at *Path, sc scope) value.Value {
	if c, ok := r.T.Base().(*types.CaseType); ok {
		w.retiredCase(r, c, at, sc)
	}
	fsc := sc.part()
	fsc.env, fsc.dep = sc.env.within(r, t), nil
	if w.stage != nil {
		if bound := w.stage.Params(r); len(bound) > 0 {
			w.voidRec()
			fsc.env.params = maps.Clone(bound) // the instance's own arguments (TYPES.md §11.1)
		}
	}
	fields := parts{from: r.Fields}
	for i, f := range Fields(r.T) {
		if i < len(r.Fields) {
			fsc.field = f.Name
			fields.set(i, w.walk(r.Fields[i], f.Type, at.Field(f.Name), fsc))
		}
	}
	if !fields.changed() {
		return r
	}
	return w.moved(r, &value.Record{T: r.T, Fields: fields.all(), Set: r.Set, Ident: r.Ident, P: r.P})
}

// list walks the elements; a keyed list's are paths by key, and its keys are unique.
func (w *walker) list(l *value.List, t types.Type, at *Path, sc scope) value.Value {
	lt := listOf(t, l.T)
	if lt == nil {
		return l
	}
	elems := parts{from: l.Elems}
	esc := sc.part()
	for i, e := range l.Elems {
		p := at.Index(i)
		r, isRec := e.(*value.Record)
		if isRec && lt.KeyedBy != nil && r.Ident != nil {
			p = at.Key(r.Ident.Key)
		}
		if isRec {
			elems.set(i, w.entryAt(at, r, lt.Elem, p, esc))
		} else {
			elems.set(i, w.walk(e, lt.Elem, p, esc))
		}
	}
	if lt.KeyedBy != nil {
		w.uniqueKeys(l, lt, at)
	}
	ct := w.computedType(l.T, t, sc)
	if !elems.changed() && ct == l.T {
		return l
	}
	return w.moved(l, &value.List{T: ct, Elems: elems.all(), P: l.P})
}

// computedType is the container type t with its applications computed, stored when none (TYPES.md §7.5).
func (w *walker) computedType(stored, t types.Type, sc scope) types.Type {
	if sc.dep != nil || t == nil || !holdsApp(t) {
		return stored
	}
	return w.substitute(t, sc.env)
}

// mapping walks each key, then its value, a dependent map's with its binder bound (TYPES.md §11.5).
func (w *walker) mapping(m *value.Map, t types.Type, at *Path, sc scope) value.Value {
	var kt, vt types.Type
	binder := ""
	switch mt := underOf(t, m.T).(type) {
	case *types.MapType:
		kt, vt = mt.Key, mt.Value
	case *types.DepMapType:
		vt, binder = mt.Value, mt.Binder
	}
	keys, vals := parts{from: m.Keys}, parts{from: m.Vals}
	ksc := sc.part()
	for i, k := range m.Keys {
		p := w.keyPath(at, k, kt, sc.env)
		keys.set(i, w.walk(k, kt, p, ksc))
		if i < len(m.Vals) {
			vsc := ksc
			if binder != "" {
				vsc.env = sc.env.bind(binder, keys.at(i))
			}
			vals.set(i, w.walk(m.Vals[i], vt, p, vsc))
		}
	}
	ct := w.computedType(m.T, t, sc)
	if !keys.changed() && !vals.changed() && ct == m.T {
		w.uniqueWire(m, at)
		return m
	}
	out := &value.Map{T: ct, Keys: keys.all(), Vals: vals.all(), P: m.P}
	if keys.changed() {
		w.uniqueResolved(out, at)
	}
	w.uniqueWire(out, at)
	return w.moved(m, out)
}

// table walks each entry in the scope of that entry, then checks its @stable values.
func (w *walker) table(tv *value.Table, t types.Type, at *Path, sc scope) value.Value {
	tt, ok := underOf(t, tv.T).(*types.TableType)
	if !ok {
		return tv
	}
	var entries []*value.Record
	esc := sc.part()
	for i, e := range tv.Entries {
		if e == nil || e.Ident == nil {
			continue
		}
		p := at.Entry(e.Ident.Key)
		esc.entry, esc.retired = p.String(), sc.retired || e.Ident.Retired
		ne, isRec := w.entryAt(at, e, tt.Elem, p, esc).(*value.Record)
		if isRec && ne != e && entries == nil {
			entries = slices.Clone(tv.Entries)
		}
		if isRec && entries != nil {
			entries[i] = ne
		}
	}
	if tt.Stable {
		w.uniqueStable(tv, tt, at)
	}
	if entries == nil {
		return tv
	}
	return w.moved(tv, &value.Table{T: tv.T, Entries: entries, P: tv.P})
}

func (w *walker) pair(p *value.Pair, t types.Type, at *Path, sc scope) value.Value {
	var a, b types.Type
	if pt, ok := underOf(t, p.T).(*types.PairType); ok {
		a, b = pt.A, pt.B
	}
	psc := sc.part()
	na, nb := w.walk(p.A, a, at, psc), w.walk(p.B, b, at, psc)
	if na == p.A && nb == p.B {
		return p
	}
	return w.moved(p, &value.Pair{T: p.T, A: na, B: nb, P: p.P})
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
