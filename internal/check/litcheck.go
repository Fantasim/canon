package check

import (
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// rawValue is a value argument already in its canonical text (STDLIB.md §9: strings quoted).
type rawValue string

func (r rawValue) CanonText() string { return string(r) }

// checkLiteral reports statically a literal that breaks what it is stored into: a negated literal as the value it negates to, a key of a ref as a literal of its chain's leaf key field (TYPES.md §5.3, §7.1, §7.4; DECISIONS 456).
func (c *checker) checkLiteral(env *env, e syntax.Expr, want types.Type) {
	at := inner(e)
	lit := valueOf(at)
	if lit == nil || at == env.negand {
		return
	}
	for t := want; t != nil; {
		switch x := t.(type) {
		case *types.Alias:
			t = x.Def
		case *types.OptionalType:
			t = x.Elem
		case *types.LitUnionType:
			t = c.otherThan(x, lit)
		case *types.RefType:
			f := c.leafField(x)
			if f == nil {
				return
			}
			env, t = env.storing(c.fieldObjects[f], at), c.fieldType(f)
		case *types.Refined:
			if !c.literalRefinement(env, at, lit, x) {
				return
			}
			t = x.Of
		case types.Basic:
			c.literalLimits(env, at, lit, x)
			return
		default:
			return
		}
	}
}

// otherThan is the alternative of u a literal is a value of: none when it is one of u's literals, which wins, or when u is refused, the error type (TYPES.md §1, §13.2).
func (c *checker) otherThan(u *types.LitUnionType, lit syntax.Expr) types.Type {
	if s, isStr := stringLiteral(lit); (isStr && slices.Contains(u.Literals, s)) || c.refusedUnion(u) {
		return nil
	}
	return u.Of
}

// valueOf is the literal e stands for: the literal token under parentheses, or a numeric literal under negations, negated (TYPES.md §5.3, §7.1); nil for anything else, a negation past 64 bits included (negationRange's).
func valueOf(e syntax.Expr) syntax.Expr {
	u, ok := inner(e).(*syntax.UnaryExpr)
	if !ok {
		return literalOf(e)
	}
	v := valueOf(u.X)
	if u.Op != syntax.TokMinus || !negatable(v) {
		return nil
	}
	return negated(v)
}

// negatable reports a numeric literal whose operand and negation both hold in 64 bits (DECISIONS 460).
func negatable(lit syntax.Expr) bool {
	switch l := lit.(type) {
	case *syntax.IntLit:
		return l.Value.IsInt64() && l.Value.Int64() != math.MinInt64
	case *syntax.DurationLit:
		return l.Millis != math.MinInt64
	}
	return lit != nil
}

// negationRange is E3201 for a negation of a literal past the 64 bits expressions compute in, want its expected type: on an Int operand, which is no stored value, else on the value it yields (TYPES.md §7.2; DECISIONS 460).
func (c *checker) negationRange(env *env, e *syntax.UnaryExpr, want types.Type) {
	if l, isInt := literalOf(e.X).(*syntax.IntLit); isInt && !l.Value.IsInt64() {
		c.limitFinding(env.storingAt(nil, e), inner(e.X), types.IntType, l.Value.String())
		return
	}
	switch v := valueOf(e.X).(type) {
	case *syntax.IntLit:
		if n := new(big.Int).Neg(v.Value); !n.IsInt64() {
			c.limitFinding(storedAs(env, e, want, types.IntType), e, types.IntType, n.String())
		}
	case *syntax.DurationLit:
		if v.Millis == math.MinInt64 { // types.DurationText takes an int64, which 2^63 ms is not: -2^63 ms's text without its sign is its magnitude's
			c.limitFinding(storedAs(env, e, want, types.DurationType), e, types.DurationType, strings.TrimPrefix(types.DurationText(v.Millis), hyphen))
		}
	}
}

// storedAs is env for a finding at e naming the type limit: its declaration's note only when limit is the type it stores, want (API.md §4.4 F12).
func storedAs(env *env, e syntax.Expr, want types.Type, limit types.Basic) *env {
	if b, isBasic := unwrap(want).(types.Basic); isBasic && b == limit {
		return env
	}
	return env.storingAt(nil, e)
}

// negated is a copy of a numeric literal holding its negation, nil for anything else.
func negated(lit syntax.Expr) syntax.Expr {
	switch l := lit.(type) {
	case *syntax.IntLit:
		n := *l
		n.Value = new(big.Int).Neg(l.Value)
		return &n
	case *syntax.FloatLit:
		n := *l
		n.Neg = !l.Neg
		return &n
	case *syntax.DurationLit:
		n := *l
		n.Millis = -l.Millis
		return &n
	}
	return nil
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

// related notes the field, let or parameter whose type the value at failed, `label: String(1..)` (API.md F12).
func (c *checker) related(env *env, at syntax.Expr, b *diag.Builder) *diag.Builder {
	s := env.site
	if s == nil || s.typ == nil || inner(at) != env.siteAt {
		return b
	}
	span := s.file.Span(s.typ)
	if s.name != nil {
		span = s.file.Span(s.name).Cover(span)
	}
	return b.Related(span, diag.NoteSource(span))
}

// declaredAs is the name and written type of a field, let or parameter declaration.
func declaredAs(d syntax.Node) (*syntax.Ident, syntax.Type) {
	switch d := d.(type) {
	case *syntax.FieldDecl:
		return d.Name, d.Type
	case *syntax.LetDecl:
		return d.Name, d.Type
	case *syntax.Param:
		return d.Name, d.Type
	}
	return nil, nil
}

// literalLimits is E3201 at at for the literal value lit outside its sized type or the Duration range (TYPES.md §7.2).
func (c *checker) literalLimits(env *env, at, lit syntax.Expr, b types.Basic) {
	lo, hi, ok := b.Limits()
	if !ok {
		return
	}
	switch l := lit.(type) {
	case *syntax.IntLit:
		if b.K == types.Int && !inInt64(l.Value, lo, hi) {
			c.limitFinding(env, at, b, l.Value.String())
		}
	case *syntax.DurationLit:
		if b.K == types.Duration && (l.Millis < lo || l.Millis > hi) {
			c.limitFinding(env, at, b, types.DurationText(l.Millis))
		}
	}
}

// limitFinding is E3201 at at: text outside the sized type b.
func (c *checker) limitFinding(env *env, at syntax.Expr, b types.Basic, text string) {
	c.report(env, c.related(env, at, diag.E3201.At(env.span(at), rawValue(text), b)))
}

// patternFinding is E3205 at at: the quoted text breaks r's pattern.
func (c *checker) patternFinding(env *env, at syntax.Expr, r *types.Refined, text string) {
	c.report(env, c.related(env, at, diag.E3205.At(env.span(at), rawValue(text), r.Pattern.String())))
}

// rangeFinding is E3204 at at: text outside r's range.
func (c *checker) rangeFinding(env *env, at syntax.Expr, r *types.Refined, text string) {
	c.report(env, c.related(env, at, diag.E3204.At(env.span(at), rawValue(text), c.boundSpans[r.Range].span())))
}

// stored is f as a value of type t is compared once stored: a Float32 rounds to binary32 unless that overflows, which storage reports as E3202 and keeps (TYPES.md §7.3, §7.4; eval/store.go).
func stored(t types.Type, f float64) float64 {
	b, ok := t.Base().(types.Basic)
	if !ok || b.K != types.Float || b.Bits != types.Float32Type.Bits {
		return f
	}
	if r := float64(float32(f)); !math.IsInf(r, 0) {
		return r
	}
	return f
}

func inInt64(v *big.Int, lo, hi int64) bool {
	return v.IsInt64() && v.Int64() >= lo && v.Int64() <= hi
}

// literalRefinement checks the literal value lit, written at at, against one written refinement;
// false once it reported, so a literal gets one finding.
func (c *checker) literalRefinement(env *env, at, lit syntax.Expr, r *types.Refined) bool {
	if r.Pattern != nil {
		s, isStr := stringLiteral(lit)
		if isStr && !r.Pattern.MatchString(s) {
			c.patternFinding(env, at, r, types.QuoteString(s))
			return false
		}
	}
	if r.Range == nil {
		return true
	}
	v, text, ok := measure(lit, r.Of.Base().Kind())
	v.F = stored(r.Of, v.F)
	if !ok || inBound(v, r.Range, r.Of.Base().Kind()) {
		return true
	}
	c.rangeFinding(env, at, r, text)
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
