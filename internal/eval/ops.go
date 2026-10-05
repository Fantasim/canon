package eval

import (
	"math"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/source"
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
	neg := r.std(std.Neg(r.host(), v, r.prov(e, value.ProvComputed)))
	if _, isLit := x.X.(*syntax.IntLit); isLit {
		return r.literal(neg) // a literal token with a leading - (TYPES.md §5.3)
	}
	return neg
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
	if r.cmp != nil && r.cmp.at == e {
		r.cmp.left, r.cmp.right = a, b
	}
	if x.Op == syntax.KwIn {
		return r.inOp(x, a, b)
	}
	return r.binop(x.Op, a, b, e, r.typeOf(e))
}

// inOp is `x in xs`, an element or a key as the static type of x decides (STDLIB.md §5).
func (r *run) inOp(x *syntax.BinaryExpr, a, b value.Value) value.Value {
	r.site = r.span(x)
	in, ok := r.memberOf(a, b, r.typeOf(x.X))
	return r.boolOr(in, ok, r.prov(x, value.ProvComputed))
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
	at := func() source.Span { return r.span(n) }
	if op == syntax.TokEq || op == syntax.TokNe {
		eq, ok := r.equal(a, b, at)
		return r.boolOr(eq == (op == syntax.TokEq), ok, p)
	}
	if res, ok := order(op, a, b); ok {
		return &value.Bool{V: res, P: p}
	}
	if size, ok := concatLen(op, a, b); ok {
		if !r.spend(size, func() source.Span { return r.span(n) }) {
			return nil
		}
		res := concat(a, b, t, p)
		if l, ok := res.(*value.List); ok {
			return r.settledList(l, a, b)
		}
		return res
	}
	aop, ok := arithOps[op]
	if !ok {
		r.bug(n)
		return nil
	}
	r.site = r.span(n)
	if r.fr.ts && overflows(aop, a, b) {
		r.tsFail()
		return nil
	}
	return r.std(std.Arith(r.host(), aop, a, b, p))
}

// comparisons are the operators compare applies.
var comparisons = map[syntax.TokenKind]bool{
	syntax.TokEq: true, syntax.TokNe: true, syntax.TokLt: true, syntax.TokLe: true,
	syntax.TokGt: true, syntax.TokGe: true,
}

// boolOr is a Bool of b, or nil once the root aborted.
func (r *run) boolOr(b, ok bool, p *value.Prov) value.Value {
	if !ok {
		return nil
	}
	return &value.Bool{V: b, P: p}
}

// order is an ordering comparison, when op is one.
func order(op syntax.TokenKind, a, b value.Value) (bool, bool) {
	switch op {
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

// concatLen is the length of `a + b` on strings (bytes) or lists (elements), one step each
// (DECISIONS 195); false for another operator or other operands.
func concatLen(op syntax.TokenKind, a, b value.Value) (int, bool) {
	if op != syntax.TokPlus {
		return 0, false
	}
	switch x := a.(type) {
	case *value.Str:
		if y, ok := b.(*value.Str); ok {
			return len(x.V) + len(y.V), true
		}
	case *value.List:
		return len(x.Elems) + len(std.Elems(b)), true
	}
	return 0, false
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

// memberOf is `x in xs`, x typed xt, equalities charged at r.site (TYPES.md §6.5, DECISIONS 197).
func (r *run) memberOf(x, coll value.Value, xt types.Type) (bool, bool) {
	if isNone(x) {
		return false, true
	}
	switch c := coll.(type) {
	case *value.Range:
		i, ok := x.(*value.Int)
		return ok && std.InRange(c, i.V), true
	case *value.Map:
		i, ok := std.MapIndex(r.host(), c, x)
		return i >= 0, ok
	case *value.List:
		x = r.valueAs(x, elemOf(c.T))
	}
	switch k, how, ok := std.MemberKey(r.host(), coll, x, xt); {
	case !ok:
		return false, false
	case how == std.OperandMissing:
		return false, true
	case how == std.OperandKey:
		_, found := r.ev.entry(coll, k)
		return found, true
	}
	for _, e := range std.Elems(coll) {
		if eq, ok := r.host().Equal(e, x); !ok || eq {
			return eq, ok
		}
	}
	return false, true
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
