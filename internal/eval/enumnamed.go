package eval

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// enumNamed is `E.named(s)`, the member named s or none, for the call's step and s's (STDLIB.md §3, EVALUATION.md §12.1).
func (r *run) enumNamed(x *syntax.CallExpr, s *syntax.SelectorExpr) value.Value {
	e := r.enumOf(s.X)
	if e == nil {
		return nil
	}
	if len(x.Args) != 1 {
		r.bug(x)
		return nil
	}
	v := r.eval(x.Args[0].Value)
	if v == nil {
		return nil
	}
	str, ok := v.(*value.Str)
	if !ok {
		r.bug(x)
		return nil
	}
	p := r.prov(x, value.ProvComputed)
	i, found := r.ev.enumIndex(e)[str.V]
	if !found {
		return &value.None{T: r.typeOf(x), P: p}
	}
	return &value.Member{Enum: e, Index: i, P: p}
}

// enumIndex is t's member indexes by name, kept by the root evaluator; one built from a shorter Members is rebuilt (TYPES.md §4.3).
func (e *Evaluator) enumIndex(t *types.EnumType) map[string]int {
	for e.parent != nil {
		e = e.parent
	}
	if idx, ok := e.enumNames[t]; ok && len(idx) == len(t.Members) {
		return idx
	}
	if e.enumNames == nil {
		e.enumNames = map[*types.EnumType]map[string]int{}
	}
	idx := make(map[string]int, len(t.Members))
	for i, m := range t.Members {
		idx[m.Name] = i
	}
	e.enumNames[t] = idx
	return idx
}
