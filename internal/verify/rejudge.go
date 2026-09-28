package verify

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// rejudge runs on v, a copy verified in another scope, only E3506 and E3502, which read it (LOCK.md §4.3).
func (w *walker) rejudge(v value.Value, at *Path, sc scope) {
	switch x := v.(type) {
	case *value.Member:
		w.member(x, at, sc)
	case *value.Ref:
		w.rejudgeRef(x, sc, at)
	case *value.Record:
		w.rejudgeRecord(x, at, sc)
	case *value.List:
		keyed := listOf(nil, x.T) != nil && listOf(nil, x.T).KeyedBy != nil
		for i, e := range x.Elems {
			p := at.Index(i)
			if r, ok := e.(*value.Record); ok && keyed && r.Ident != nil {
				p = at.Key(r.Ident.Key)
			}
			w.rejudge(e, p, sc)
		}
	case *value.Map:
		w.rejudgeMap(x, at, sc)
	case *value.Table:
		w.rejudgeTable(x, at, sc)
	case *value.Pair:
		w.rejudge(x.A, at, sc)
		w.rejudge(x.B, at, sc)
	}
}

func (w *walker) rejudgeRecord(r *value.Record, at *Path, sc scope) {
	if c, ok := r.T.Base().(*types.CaseType); ok {
		w.retiredCase(r, c, at, sc)
	}
	for i, f := range Fields(r.T) {
		if i < len(r.Fields) {
			w.rejudge(r.Fields[i], at.Field(f.Name), sc)
		}
	}
}

func (w *walker) rejudgeMap(m *value.Map, at *Path, sc scope) {
	for i, k := range m.Keys {
		p := at.Key(KeyOf(k))
		w.rejudge(k, p, sc)
		if i < len(m.Vals) {
			w.rejudge(m.Vals[i], p, sc)
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
		w.rejudge(e, p, esc)
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
