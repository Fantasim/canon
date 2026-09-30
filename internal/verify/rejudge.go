package verify

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// rejudge runs on v, a copy verified in another scope and declared t there, only E3506 and E3502, which read it (LOCK.md §4.3).
func (w *walker) rejudge(v value.Value, t types.Type, at *Path, sc scope) {
	switch x := v.(type) {
	case *value.Member:
		w.member(x, at, sc)
	case *value.Ref:
		w.rejudgeRef(x, sc, at)
	case *value.Record:
		w.rejudgeRecord(x, at, sc)
	case *value.List:
		w.rejudgeList(x, t, at, sc)
	case *value.Map:
		w.rejudgeMap(x, t, at, sc)
	case *value.Table:
		w.rejudgeTable(x, at, sc)
	case *value.Pair:
		var a, b types.Type
		if pt, ok := Declared(t).(*types.PairType); ok {
			a, b = pt.A, pt.B
		}
		w.rejudge(x.A, a, at, sc)
		w.rejudge(x.B, b, at, sc)
	}
}

func (w *walker) rejudgeRecord(r *value.Record, at *Path, sc scope) {
	if c, ok := r.T.Base().(*types.CaseType); ok {
		w.retiredCase(r, c, at, sc)
	}
	for i, f := range Fields(r.T) {
		if i < len(r.Fields) {
			w.rejudge(r.Fields[i], f.Type, at.Field(f.Name), sc)
		}
	}
}

func (w *walker) rejudgeList(l *value.List, t types.Type, at *Path, sc scope) {
	lt, et := ListAt(t, l)
	keyed := lt != nil && lt.KeyedBy != nil
	for i, e := range l.Elems {
		p := at.Index(i)
		if r, ok := e.(*value.Record); ok && keyed && r.Ident != nil {
			p = at.Key(r.Ident.Key)
		}
		w.rejudge(e, et, p, sc)
	}
}

// rejudgeMap names each entry by the key type declared where the map is (API.md P9, log-2026-09-29 M4 U13-r).
func (w *walker) rejudgeMap(m *value.Map, t types.Type, at *Path, sc scope) {
	kt, vt := KeyTypeAt(t, m), types.Type(nil)
	if mt, ok := declaredMap(t); ok {
		vt = mt.Value
	}
	for i, k := range m.Keys {
		p := at.MapKey(k, kt)
		w.rejudge(k, nil, p, sc)
		if i < len(m.Vals) {
			w.rejudge(m.Vals[i], vt, p, sc)
		}
	}
}

// rejudgeTable judges each entry in the scope of that entry.
func (w *walker) rejudgeTable(tv *value.Table, at *Path, sc scope) {
	for _, e := range tv.Entries {
		if e == nil || e.Ident == nil {
			continue
		}
		p := at.Entry(e.Ident.Key)
		esc := sc
		esc.entry, esc.retired = p.String(), sc.retired || e.Ident.Retired
		w.rejudge(e, nil, p, esc)
	}
}

// rejudgeRef judges a ref's target again; a missing target was reported with the copy (E3501).
func (w *walker) rejudgeRef(r *value.Ref, sc scope, at *Path) {
	rt, ok := r.T.Base().(*types.RefType)
	if !ok || rt.Target == nil {
		return
	}
	if entries, how := w.collection(r, rt.Target); how == reached {
		if target, found := entries[r.Key]; found {
			w.retiredTarget(r, rt, target, sc, at)
		}
	}
}

// retiredTarget is E3502: a live table entry references a retired one (TYPES.md §10.3, LOCK.md §4.3).
func (w *walker) retiredTarget(r *value.Ref, rt *types.RefType, target *value.Record, sc scope, at *Path) {
	if target.Ident.Retired && sc.entry != "" && !sc.retired {
		s := SiteOf(r)
		w.flagScoped(s, w.src.related(diag.E3502.At(s.Span, r, sc.entry), rt), r, at)
	}
}
