package eval

import (
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

// reprov is v with a new origin p, without v's history but with its position mark; a composite copied only when composites (EVALUATION.md §4.2, §13.1).
func (e *Evaluator) reprov(v value.Value, p *value.Prov, composites bool) value.Value {
	if value.IsPosition(v) {
		p = e.positionProv(p)
	}
	out := reprovValue(v, p, composites)
	if rec, ok := v.(*value.Record); ok {
		if cp, isRec := out.(*value.Record); isRec {
			e.copiedFrom(rec, cp)
		}
	}
	return e.mark(v, out)
}

func reprovValue(v value.Value, p *value.Prov, composites bool) value.Value {
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
	}
	if !composites {
		return v
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

// carry gives a value rebuilt from another, the same value passed along, its invalid mark and history (EVALUATION.md §13).
func (e *Evaluator) carry(from, to value.Value) value.Value {
	if old := e.before(from); old != nil && from != to && to != nil {
		e.history[to] = old
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
	return to
}
