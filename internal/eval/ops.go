package eval

import (
	"math"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// arithOps maps an operator token to its arithmetic operator (TYPES.md §7.1).
var arithOps = map[syntax.TokenKind]std.Op{
	syntax.TokPlus: std.OpAdd, syntax.TokMinus: std.OpSub, syntax.TokStar: std.OpMul,
	syntax.TokSlash: std.OpDiv, syntax.TokPercent: std.OpMod,
}

// evalUnary is `-x` (E4101 on the smallest Int or Duration) or `not x`.
func evalUnary(r *run, e syntax.Expr, _ *vpath) value.Value {
	x := e.(*syntax.UnaryExpr)
	v := r.eval(x.X)
	if v == nil {
		return nil
	}
	if b, ok := v.(*value.Bool); ok && x.Op == syntax.KwNot {
		return &value.Bool{V: !b.V, P: r.prov(e, value.ProvComputed)}
	}
	r.site = r.span(e)
	return r.std(std.Neg(r.host(), v, r.prov(e, value.ProvComputed)))
}

// evalBinary evaluates operands left to right, `and`, `or`, `??` lazily (EVALUATION.md §2.2).
func evalBinary(r *run, e syntax.Expr, _ *vpath) value.Value {
	x := e.(*syntax.BinaryExpr)
	switch x.Op {
	case syntax.KwAnd, syntax.KwOr:
		return r.logic(x)
	case syntax.TokCoalesce:
		a := r.eval(x.X)
		if isNone(a) {
			return r.eval(x.Y)
		}
		return a
	default:
	}
	a := r.eval(x.X)
	if a == nil {
		return nil
	}
	b := r.eval(x.Y)
	if b == nil {
		return nil
	}
	return r.binop(x.Op, a, b, e, r.typeOf(e))
}

func (r *run) logic(x *syntax.BinaryExpr) value.Value {
	a, ok := r.truth(x.X)
	if !ok {
		return nil
	}
	if a == (x.Op == syntax.KwOr) {
		return &value.Bool{V: a, P: r.prov(x, value.ProvComputed)}
	}
	b, ok := r.truth(x.Y)
	if !ok {
		return nil
	}
	return &value.Bool{V: b, P: r.prov(x, value.ProvComputed)}
}

// binop applies a binary operator; t types a list concatenation (TYPES.md §7.1, §7.5).
func (r *run) binop(op syntax.TokenKind, a, b value.Value, n syntax.Node, t types.Type) value.Value {
	p := r.prov(n, value.ProvComputed)
	if res, ok := compare(op, a, b); ok {
		return &value.Bool{V: res, P: p}
	}
	if op == syntax.KwIn {
		return &value.Bool{V: r.memberOf(a, b), P: p}
	}
	if op == syntax.TokPlus {
		if v := concat(a, b, t, p); v != nil {
			return v
		}
	}
	aop, ok := arithOps[op]
	if !ok {
		r.bug(n)
		return nil
	}
	r.site = r.span(n)
	return r.std(std.Arith(r.host(), aop, a, b, p))
}

// compare is ==, != and the orderings, when op is one.
func compare(op syntax.TokenKind, a, b value.Value) (bool, bool) {
	switch op {
	case syntax.TokEq:
		return value.Equal(a, b), true
	case syntax.TokNe:
		return !value.Equal(a, b), true
	case syntax.TokLt:
		return std.Less(a, b), true
	case syntax.TokLe:
		return !std.Less(b, a), true
	case syntax.TokGt:
		return std.Less(b, a), true
	case syntax.TokGe:
		return !std.Less(a, b), true
	default:
	}
	return false, false
}

// concat is `+` on strings and lists (a plain list of the join, identities kept).
func concat(a, b value.Value, t types.Type, p *value.Prov) value.Value {
	switch x := a.(type) {
	case *value.Str:
		if y, ok := b.(*value.Str); ok {
			return &value.Str{V: x.V + y.V, T: types.StringType, P: p}
		}
	case *value.List:
		elems := append(append([]value.Value(nil), x.Elems...), std.Elems(b)...)
		return &value.List{T: t, Elems: elems, P: p}
	}
	return nil
}

// memberOf is `x in xs` (TYPES.md §6.5, STDLIB.md §5).
func (r *run) memberOf(x, coll value.Value) bool {
	if isNone(x) {
		return false
	}
	switch c := coll.(type) {
	case *value.Range:
		i, ok := x.(*value.Int)
		return ok && std.InRange(c, i.V)
	case *value.Map:
		_, ok := c.Get(x)
		return ok
	}
	if keyedColl(coll) {
		if _, isRec := x.(*value.Record); !isRec {
			if _, isRef := x.(*value.Ref); !isRef {
				k, _ := std.KeyOf(x)
				_, found := r.ev.entry(coll, k)
				return found
			}
		}
	}
	for _, e := range std.Elems(coll) {
		if value.Equal(e, x) {
			return true
		}
	}
	return false
}

func keyedColl(v value.Value) bool {
	switch x := v.(type) {
	case *value.Table:
		return true
	case *value.List:
		return std.Keyed(x)
	}
	return false
}

// evalIs is `x is c`: x holds case c; none is no case (TYPES.md §8.3).
func evalIs(r *run, e syntax.Expr, _ *vpath) value.Value {
	x := e.(*syntax.IsExpr)
	v := r.eval(x.X)
	if v == nil {
		return nil
	}
	parts := x.Target.Parts
	obj := r.ev.info.NameUses[parts[len(parts)-1]]
	rec, isRec := v.(*value.Record)
	yes := isRec && obj != nil && rec.T.Base() == obj.Type().Base()
	return &value.Bool{V: yes, P: r.prov(e, value.ProvComputed)}
}

// evalRange is `a..b`, `a..=b` (E4101 when b is the largest Int) or `a..` (TYPES.md §13.3).
func evalRange(r *run, e syntax.Expr, _ *vpath) value.Value {
	x := e.(*syntax.RangeExpr)
	lo, hi, ok := r.bounds(x)
	if !ok {
		return nil
	}
	if x.Op == syntax.TokRangeIncl {
		if hi == math.MaxInt64 {
			r.fail(diag.E4101.AtInteger(r.span(e), r.span(e)))
			return nil
		}
		hi++
	}
	return &value.Range{Start: lo, End: hi, HasEnd: x.Hi != nil, P: r.prov(e, value.ProvComputed)}
}

// bounds evaluates a range's bounds, 0 for an absent one.
func (r *run) bounds(x *syntax.RangeExpr) (int64, int64, bool) {
	lo, okLo := r.bound(x.Lo)
	hi, okHi := r.bound(x.Hi)
	return lo, hi, okLo && okHi
}

// bound is one bound of a range or slice: 0 when absent.
func (r *run) bound(e syntax.Expr) (int64, bool) {
	if e == nil || r.failed {
		return 0, !r.failed
	}
	v, ok := r.eval(e).(*value.Int)
	if !ok {
		r.bug(e)
		return 0, false
	}
	return v.V, true
}
