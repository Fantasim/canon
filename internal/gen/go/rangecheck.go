package gogen

import (
	"fmt"
	"go/token"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// checkRange refuses the value x just read as t, when it lies outside t's range or length refinement, failing at loc (CODEGEN.md §5.15); a literal of a union passes. It binds x to a local unless it is one already, and returns the expression to use afterwards.
func (g *gen) checkRange(b *strings.Builder, t ir.TypeRef, x string, loc location) string {
	literals, base := rangeSubject(t)
	failure := ir.RangeFailure(&base)
	if failure == "" || ir.Implied(&base, false) && ir.Implied(&base, true) {
		return x
	}
	v := x
	if !token.IsIdentifier(x) {
		v = g.temp(tempValue)
		fmt.Fprintf(b, defineFormat, v, x)
	}
	cond := g.rangeCond(base, g.measure(base, v))
	if len(literals) > 0 {
		cond = strings.Join(literalTests(v, literals), orSep) + orSep + lparen + cond + rparen
	}
	fmt.Fprintf(b, failUnlessFormat, lparen+cond+rparen, g.errAt(loc, failure, ""))
	return v
}

// rangeSubject is the type whose refinement is checked and the literals that pass it: a literal union's base and its literals, else t itself.
func rangeSubject(t ir.TypeRef) (literals []string, base ir.TypeRef) {
	if t.Kind == types.LitUnion && t.Elem != nil {
		return t.Literals, *t.Elem
	}
	return nil, t
}

// literalTests are the tests that v is one of a union's literals.
func literalTests(v string, literals []string) []string {
	out := make([]string, len(literals))
	for i, lit := range literals {
		out[i] = fmt.Sprintf(condFormat, v, opText[ir.OpEq], strconv.Quote(lit))
	}
	return out
}

// rangeCond is the test that the measure of a value of type t is within t's refinement: each side not implied by the stored type, lower first (CODEGEN.md §5.15).
func (g *gen) rangeCond(t ir.TypeRef, measured string) string {
	r := t.Range
	var conds []string
	if r.HasLo && !ir.Implied(&t, false) {
		conds = append(conds, fmt.Sprintf(condFormat, measured, opText[ir.OpGe], g.boundLit(t, r.Lo)))
	}
	if r.HasHi && !ir.Implied(&t, true) {
		op := opText[ir.OpLt]
		if r.HiIncluded {
			op = opText[ir.OpLe]
		}
		conds = append(conds, fmt.Sprintf(condFormat, measured, op, g.boundLit(t, r.Hi)))
	}
	return strings.Join(conds, andSep)
}

// measure is what a refinement of t compares in the local v: the integer, the binary64 value, the milliseconds, the bytes or the element count (CODEGEN.md §5.15).
func (g *gen) measure(t ir.TypeRef, v string) string {
	switch {
	case t.Kind == types.Int && g.goType(t) == goInt64, t.Kind == types.Float && t.Bits != float32Bits:
		return v
	case t.Kind == types.Int:
		return goInt64 + lparen + v + rparen
	case t.Kind == types.Float:
		return goFloat64 + lparen + v + rparen
	case t.Kind == types.Duration:
		return v + durationMillis
	case t.Kind == types.String:
		return goInt64 + lparen + lenCall + v + rparen + rparen
	}
	return goInt64 + lparen + v + dot + ir.GoLen + callSuffix + rparen
}

// boundLit is one bound of t as a Go literal: a binary64 for a Float (an untyped constant, compared with a float64), else an integer.
func (g *gen) boundLit(t ir.TypeRef, l types.Limit) string {
	if t.Kind == types.Float {
		return floatBound(l.F)
	}
	return intLit(l.I)
}

// floatBound is f as a Go constant that reads back as the same binary64: the shortest literal, a negative zero as 0 (Copysign is no constant).
func floatBound(f float64) string {
	if f == 0 {
		return zeroLit
	}
	return strconv.FormatFloat(f, floatFormat, shortest, int64Bits)
}

// rowsRange is the test and the failure text for the count of a data file's rows, held in the slice values, once their duplicates are checked (CODEGEN.md §5.15); empty when the rows have no refinement to check.
func (g *gen) rowsRange(v *ir.Value, values string) (cond, fail string) {
	failure := ir.RangeFailure(&v.Type)
	if failure == "" {
		return "", ""
	}
	cond = g.rangeCond(v.Type, goInt64+lparen+lenCall+values+rparen+rparen)
	if cond == "" {
		return "", ""
	}
	return cond, escapeVerbs(failure)
}
