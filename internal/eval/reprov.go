package eval

import (
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

// reprov is v with a new origin p, a composite shallow, without v's history but keeping its position mark, its written mark and a symbol's written site (EVALUATION.md §13.1).
func (e *Evaluator) reprov(v value.Value, p *value.Prov) value.Value {
	if value.IsPosition(v) {
		p = e.positionProv(p)
	}
	out := reprovValue(v, p)
	if rec, ok := v.(*value.Record); ok {
		if cp, isRec := out.(*value.Record); isRec {
			e.copiedFrom(rec, cp)
		}
	}
	e.keepWritten(v, out)
	return e.mark(v, out)
}

// keepWritten gives out, v under a fresh provenance, v's written mark and a symbol's written site; a shared one would let the last site win (TYPES.md §11.4).
func (e *Evaluator) keepWritten(v, out value.Value) {
	if out == v || out == nil {
		return
	}
	if e.isWritten(v) {
		e.written[out], e.gens.written = true, e.gens.written+1
	}
	if s, ok := v.(*value.Symbol); ok && out.Prov() != nil {
		e.symSites[out.Prov()] = e.writtenAt(s.P)
	}
}

// writtenAt is where a symbol of provenance p was written: the site reprov kept for p, else p.
func (e *Evaluator) writtenAt(p *value.Prov) *value.Prov {
	for x := e; x != nil; x = x.parent {
		if site, ok := x.symSites[p]; ok {
			return site
		}
	}
	return p
}

func reprovValue(v value.Value, p *value.Prov) value.Value {
	switch x := v.(type) {
	case *value.Bool:
		return &value.Bool{V: x.V, P: p}
	case *value.Int:
		return &value.Int{V: x.V, T: x.T, P: p}
	case *value.Float:
		return &value.Float{V: x.V, T: x.T, P: p}
	case *value.Str:
		return &value.Str{V: x.V, T: x.T, P: p}
	case *value.Dur:
		return &value.Dur{Ms: x.Ms, P: p}
	case *value.Member:
		return &value.Member{Enum: x.Enum, Index: x.Index, P: p}
	case *value.None:
		return &value.None{T: x.T, P: p}
	case *value.Ref:
		return &value.Ref{T: x.T, Key: x.Key, Owner: x.Owner, P: p}
	case *value.Symbol:
		return &value.Symbol{Name: x.Name, T: x.T, P: p}
	}
	return reprovComposite(v, p)
}

func reprovComposite(v value.Value, p *value.Prov) value.Value {
	switch x := v.(type) {
	case *value.List:
		return &value.List{T: x.T, Elems: x.Elems, P: p}
	case *value.Map:
		return &value.Map{T: x.T, Keys: x.Keys, Vals: x.Vals, P: p}
	case *value.Record:
		cp := *x
		cp.P = p
		return &cp
	case *value.Table:
		return &value.Table{T: x.T, Entries: x.Entries, P: p}
	case *value.Pair:
		return &value.Pair{T: x.T, A: x.A, B: x.B, P: p}
	}
	return v
}

// siteKey is a provenance without Via or ProvExt: all it holds.
type siteKey struct {
	kind     value.ProvKind
	position bool
	span     source.Span
}

// sharedProv is the one Prov k describes, shared by the values a default or a lambda builds at
// its site run after run (DECISIONS 373).
func (e *Evaluator) sharedProv(k siteKey) *value.Prov {
	if p, ok := e.siteProvs[k]; ok {
		return p
	}
	if e.siteProvs == nil {
		e.siteProvs = map[siteKey]*value.Prov{}
	}
	p := &value.Prov{Kind: k.kind, Position: k.position, Span: k.span}
	e.siteProvs[k] = p
	return p
}

// positionProv is p's marked twin, shared when p has no Via or Ext, else a copy; p is never written (DECISIONS 379).
func (e *Evaluator) positionProv(p *value.Prov) *value.Prov {
	if p == nil || p.Position {
		return p
	}
	if p.Via != nil || p.Ext != nil {
		twin := *p
		twin.Position = true
		return &twin
	}
	return e.sharedProv(siteKey{kind: p.Kind, position: true, span: p.Span})
}

// carry gives a value rebuilt from another, the same value passed along, its invalid mark, history and link to an amendment's copy (EVALUATION.md §13, §9.3).
func (e *Evaluator) carry(from, to value.Value) value.Value {
	if from == to || to == nil {
		return e.mark(from, to)
	}
	if old := e.before(from); old != nil {
		e.history[to] = old
	}
	if e.copyOf(from) != nil {
		e.rebuilt[to] = from
	}
	return e.mark(from, to)
}

// mark gives a value built from another its invalid mark and, a record, its arguments (EVALUATION.md §7.3).
func (e *Evaluator) mark(from, to value.Value) value.Value {
	if from == to || from == nil || to == nil {
		return to
	}
	if e.Invalid(from) {
		e.invalid[to] = true
	}
	e.shareParams(from, to)
	e.lits.copied(from, to)
	return to
}
