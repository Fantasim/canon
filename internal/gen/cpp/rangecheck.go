package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// checkRefinement writes the check of l's value against its type's range or length refinement, failing at key (CODEGEN.md §5.15); a literal of a union passes.
func (g *gen) checkRefinement(depth int, key string, l leaf) {
	if line := g.rangeLine(l.t, l.dst, key); line != "" {
		g.c.lineAt(depth, line)
	}
}

// rangeLine is the check line of the value dst of type t, "" when t has no refinement or both its sides are implied.
func (g *gen) rangeLine(t ir.TypeRef, dst, key string) string {
	var literals []string
	if t.Kind == types.LitUnion && t.Elem != nil {
		literals, t = t.Literals, *t.Elem
	}
	failure := ir.RangeFailure(&t)
	cond := g.rangeCond(t, dst)
	if failure == "" || cond == "" {
		return ""
	}
	if len(literals) > 0 {
		cond = strings.Join(literalTests(dst, literals), orSep) + orSep + fmt.Sprintf(parenFormat, cond)
	}
	return fmt.Sprintf(rangeCheckFormat, cond, key, quote(failure))
}

// literalTests are the tests that dst is one of a union's literals.
func literalTests(dst string, literals []string) []string {
	out := make([]string, len(literals))
	for i, lit := range literals {
		out[i] = fmt.Sprintf(equalFormat, dst, viewLit(lit))
	}
	return out
}

// rangeCond is the test that dst is within t's refinement: each side not implied by the stored type, lower first (CODEGEN.md §5.15).
func (g *gen) rangeCond(t ir.TypeRef, dst string) string {
	r := t.Range
	if r == nil {
		return ""
	}
	measured := g.measure(t, dst)
	var conds []string
	if r.HasLo && !ir.Implied(&t, false) {
		conds = append(conds, fmt.Sprintf(compareFormat, measured, opText[ir.OpGe], g.boundLit(t, r.Lo)))
	}
	if r.HasHi && !ir.Implied(&t, true) {
		op := opText[ir.OpLt]
		if r.HiIncluded {
			op = opText[ir.OpLe]
		}
		conds = append(conds, fmt.Sprintf(compareFormat, measured, op, g.boundLit(t, r.Hi)))
	}
	return strings.Join(conds, andSep)
}

// measure is what a refinement of t compares: the integer, the binary64 value, the milliseconds, the bytes or the element count (CODEGEN.md §5.15).
func (g *gen) measure(t ir.TypeRef, dst string) string {
	switch {
	case t.Kind == types.Int:
		return fmt.Sprintf(staticCastFormat, cppInt64, dst)
	case t.Kind == types.Float && t.Bits == float32Bits:
		return fmt.Sprintf(staticCastFormat, cppDouble, dst)
	case t.Kind == types.Float:
		return dst
	case t.Kind == types.Duration:
		return fmt.Sprintf(countFormat, dst)
	case t.Kind == types.String, t.Kind == types.List && t.KeyedBy == nil:
		return fmt.Sprintf(staticCastFormat, cppInt64, fmt.Sprintf(sizeCallFormat, dst))
	}
	return fmt.Sprintf(staticCastFormat, cppInt64, fmt.Sprintf(valueLenFormat, dst))
}

// boundLit is one bound of t as a C++ literal: a binary64 for a Float, else an integer.
func (g *gen) boundLit(t ir.TypeRef, l types.Limit) string {
	if t.Kind == types.Float {
		return g.floatLit(l.F, bits64)
	}
	return intLit(l.I)
}

// rowsRange checks the count of a data file's rows, held in values, after their duplicates (CODEGEN.md §5.15).
func (g *gen) rowsRange(v *ir.Value) {
	t := v.Type
	t.KeyedBy = nil // the rows are still a vector
	if line := g.rangeLine(t, rowsValues, quote(rowsKey)); line != "" {
		g.c.lineAt(depthTwo, line)
		g.c.lineAt(depthTwo, rowsOkLine)
	}
}
