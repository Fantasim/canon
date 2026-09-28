package control

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// IsSet reports a list declared a set (C32): a `where` that is `it.isUnique()` or a conjunction
// with it as one operand, or the field's @json(bits).
func IsSet(t types.Type, enc types.Enc) bool {
	if enc == types.EncBits {
		return true
	}
	for _, p := range shape.LayersOf(t).Where {
		for _, e := range conjuncts(p.Expr) {
			if itCall(e, wordIsUnique) {
				return true
			}
		}
	}
	return false
}

// rangeForm reports the range refinement of C33 among a `where`'s conjuncts: `it.len() == 2`
// with `it[0] <= it[1]` or an equivalent; strict for `<`.
func rangeForm(t types.Type) (strict, ok bool) {
	for _, p := range shape.LayersOf(t).Where {
		pair, ordered := false, false
		for _, e := range conjuncts(p.Expr) {
			pair = pair || lenTwo(e)
			if s, is := ordering(e); is {
				ordered, strict = true, s
			}
		}
		if pair && ordered {
			return strict, true
		}
	}
	return false, false
}

// conjuncts are the operands of a chain of `and`, parentheses removed.
func conjuncts(e syntax.Expr) []syntax.Expr {
	e = shape.Unparen(e)
	if b, ok := e.(*syntax.BinaryExpr); ok && b.Op == syntax.KwAnd {
		return append(conjuncts(b.X), conjuncts(b.Y)...)
	}
	return []syntax.Expr{e}
}

// isIt reports the name `it`.
func isIt(e syntax.Expr) bool {
	id, ok := shape.Unparen(e).(*syntax.IdentExpr)
	return ok && id.Name == wordIt
}

// itCall reports `it.name()`.
func itCall(e syntax.Expr, name string) bool {
	c, ok := shape.Unparen(e).(*syntax.CallExpr)
	if !ok || len(c.Args) > 0 {
		return false
	}
	s, ok := c.Fun.(*syntax.SelectorExpr)
	return ok && s.Name != nil && s.Name.Name == name && isIt(s.X)
}

// intLit reports an integer literal equal to n.
func intLit(e syntax.Expr, n int64) bool {
	l, ok := shape.Unparen(e).(*syntax.IntLit)
	return ok && l.Value != nil && l.Value.IsInt64() && l.Value.Int64() == n
}

// itIndex reports `it[i]`.
func itIndex(e syntax.Expr, i int64) bool {
	x, ok := shape.Unparen(e).(*syntax.IndexExpr)
	return ok && isIt(x.X) && intLit(x.Index, i)
}

// lenTwo reports `it.len() == 2` or `2 == it.len()`.
func lenTwo(e syntax.Expr) bool {
	b, ok := shape.Unparen(e).(*syntax.BinaryExpr)
	if !ok || b.Op != syntax.TokEq {
		return false
	}
	return itCall(b.X, wordLen) && intLit(b.Y, rangeLen) || intLit(b.X, rangeLen) && itCall(b.Y, wordLen)
}

// ordering reports `it[0] <= it[1]`, `it[0] < it[1]`, `it[1] >= it[0]` or `it[1] > it[0]`, and
// whether it is strict.
func ordering(e syntax.Expr) (strict, ok bool) {
	b, isBin := shape.Unparen(e).(*syntax.BinaryExpr)
	if !isBin {
		return false, false
	}
	first := itIndex(b.X, 0) && itIndex(b.Y, secondIndex)
	second := itIndex(b.X, secondIndex) && itIndex(b.Y, 0)
	switch {
	case first && (b.Op == syntax.TokLe || b.Op == syntax.TokLt):
		return b.Op == syntax.TokLt, true
	case second && (b.Op == syntax.TokGe || b.Op == syntax.TokGt):
		return b.Op == syntax.TokGt, true
	}
	return false, false
}

// equalsConstant is the `c` of `it == c` or `c == it` among a `where`'s conjuncts (C34).
func equalsConstant(t types.Type) syntax.Expr {
	for _, p := range shape.LayersOf(t).Where {
		for _, e := range conjuncts(p.Expr) {
			b, ok := e.(*syntax.BinaryExpr)
			switch {
			case !ok || b.Op != syntax.TokEq:
			case isIt(b.X) && !isIt(b.Y):
				return b.Y
			case isIt(b.Y) && !isIt(b.X):
				return b.X
			}
		}
	}
	return nil
}
