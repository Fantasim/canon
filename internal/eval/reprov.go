package eval

import "github.com/fantasim/canonlang/internal/value"

// reprov is v with a new origin p, without v's history; a composite copied only when composites (EVALUATION.md §4.2).
func (e *Evaluator) reprov(v value.Value, p *value.Prov, composites bool) value.Value {
	return e.mark(v, reprovValue(v, p, composites))
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

// carry gives a value rebuilt from another, the same value passed along, its invalid mark and history (EVALUATION.md §13).
func (e *Evaluator) carry(from, to value.Value) value.Value {
	if old := e.before(from); old != nil && from != to && to != nil {
		e.history[to] = old
	}
	return e.mark(from, to)
}

// mark gives a value built from another the invalid mark the other has (EVALUATION.md §7.3).
func (e *Evaluator) mark(from, to value.Value) value.Value {
	if from != to && from != nil && to != nil && e.Invalid(from) {
		e.invalid[to] = true
	}
	return to
}
