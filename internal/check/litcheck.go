package check

import (
	"math/big"
	"strconv"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// rawValue is a value argument already in its canonical text (STDLIB.md §9: strings quoted).
type rawValue string

func (r rawValue) CanonText() string { return string(r) }

// checkLiteral reports statically a literal that breaks what it is stored into (TYPES.md §5.3, §7.4).
func (c *checker) checkLiteral(env *env, e syntax.Expr, want types.Type) {
	lit := literalOf(e)
	if lit == nil {
		return
	}
	for t := want; t != nil; {
		switch x := t.(type) {
		case *types.Alias:
			t = x.Def
		case *types.OptionalType:
			t = x.Elem
		case *types.Refined:
			if !c.literalRefinement(env, lit, x) {
				return
			}
			t = x.Of
		case types.Basic:
			c.literalLimits(env, lit, x)
			return
		default:
			return
		}
	}
}

// literalOf is the literal token under parentheses, nil for anything else.
func literalOf(e syntax.Expr) syntax.Expr {
	for {
		switch x := e.(type) {
		case *syntax.ParenExpr:
			e = x.X
		case *syntax.IntLit, *syntax.FloatLit, *syntax.DurationLit, *syntax.RawStringLit, *syntax.ListLit:
			return x
		case *syntax.StringLit:
			if interpolationFree(x) {
				return x
			}
			return nil
		default:
			return nil
		}
	}
}

// related notes the field or let a value is stored in, `label: String(1..)` (EVALUATION.md §4.3).
func (c *checker) related(env *env, b *diag.Builder) *diag.Builder {
	if env.site == nil {
		return b
	}
	var name *syntax.Ident
	var t syntax.Type
	switch d := env.site.decl.(type) {
	case *syntax.FieldDecl:
		name, t = d.Name, d.Type
	case *syntax.LetDecl:
		name, t = d.Name, d.Type
	}
	if t == nil {
		return b
	}
	at := env.site.file.Span(name).Cover(env.site.file.Span(t))
	return b.Related(at, diag.NoteSource(at))
}

// literalLimits is E3201 for a literal outside its sized type or the Duration range (TYPES.md §7.2).
func (c *checker) literalLimits(env *env, lit syntax.Expr, b types.Basic) {
	lo, hi, ok := b.Limits()
	if !ok {
		return
	}
	switch l := lit.(type) {
	case *syntax.IntLit:
		if b.K == types.Int && !inInt64(l.Value, lo, hi) {
			c.report(env, c.related(env, diag.E3201.At(env.span(lit), rawValue(l.Value.String()), b)))
		}
	case *syntax.DurationLit:
		if b.K == types.Duration && (l.Millis < lo || l.Millis > hi) {
			c.report(env, c.related(env, diag.E3201.At(env.span(lit), rawValue(types.DurationText(l.Millis)), b)))
		}
	}
}

func inInt64(v *big.Int, lo, hi int64) bool {
	return v.IsInt64() && v.Int64() >= lo && v.Int64() <= hi
}

// literalRefinement checks one written refinement; false once it reported, so a literal gets
// one finding.
func (c *checker) literalRefinement(env *env, lit syntax.Expr, r *types.Refined) bool {
	if r.Pattern != nil {
		s, isStr := stringLiteral(lit)
		if isStr && !r.Pattern.MatchString(s) {
			c.report(env, c.related(env, diag.E3205.At(env.span(lit), rawValue(types.QuoteString(s)), r.Pattern.String())))
			return false
		}
	}
	if r.Range == nil {
		return true
	}
	v, text, ok := measure(lit, r.Of.Base().Kind())
	if !ok || inBound(v, r.Range, r.Of.Base().Kind()) {
		return true
	}
	c.report(env, c.related(env, diag.E3204.At(env.span(lit), rawValue(text), c.boundSpans[r.Range].span())))
	return false
}

// stringLiteral is the text of a constant string literal.
func stringLiteral(lit syntax.Expr) (string, bool) {
	s, ok := lit.(syntax.StrLit)
	if !ok {
		return "", false
	}
	return constText(s), true
}

// measure is what a range refinement of kind k compares for a literal: its value, or its
// length in bytes or elements; with the literal's canonical text.
func measure(lit syntax.Expr, k types.Kind) (types.Limit, string, bool) {
	switch l := lit.(type) {
	case *syntax.IntLit:
		if !l.Value.IsInt64() || (k != types.Int && k != types.Float) {
			return types.Limit{}, "", false
		}
		return types.Limit{I: l.Value.Int64(), F: float64(l.Value.Int64())}, l.Value.String(), true
	case *syntax.FloatLit:
		f := floatOf(l)
		return types.Limit{F: f}, types.FloatText(f, bits64), k == types.Float
	case *syntax.DurationLit:
		return types.Limit{I: l.Millis}, types.DurationText(l.Millis), k == types.Duration
	case *syntax.ListLit:
		return types.Limit{I: int64(len(l.Elems))}, strconv.Itoa(len(l.Elems)), k == types.List
	}
	if s, ok := stringLiteral(lit); ok && k == types.String {
		return types.Limit{I: int64(len(s))}, types.QuoteString(s), true
	}
	return types.Limit{}, "", false
}

// inBound reports that a measured literal lies in a range of kind k.
func inBound(v types.Limit, b *types.Bound, k types.Kind) bool {
	if k == types.Float {
		return (!b.HasLo || v.F >= b.Lo.F) && (!b.HasHi || v.F < b.Hi.F || (b.HiIncluded && v.F == b.Hi.F))
	}
	return (!b.HasLo || v.I >= b.Lo.I) && (!b.HasHi || v.I < b.Hi.I || (b.HiIncluded && v.I == b.Hi.I))
}

// floatOf is the float64 nearest to a float literal's exact value.
func floatOf(l *syntax.FloatLit) float64 {
	f, _ := strconv.ParseFloat(l.Coef.String()+expMark+strconv.FormatInt(l.Exp, decimalBase), bits64)
	if l.Neg {
		return -f
	}
	return f
}
