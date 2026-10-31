package tsgen

import (
	"strconv"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// rangeHelpers are the helper units that hold a value to its refinement, by the kind of the level; every other kind is a number held by decIn, or a bigint by decInBig.
var rangeHelpers = [types.Error + 1]string{
	types.String: decStrInName, types.List: decCountInName, types.Map: decCountInName, types.DepMap: decCountInName,
}

// checked is read, the expression decoding a value of type t at path, held to t's range or length refinement (CODEGEN.md §5.15); big reads an Int as a bigint. A literal of a union passes. It is read as it is when t has no refinement or the stored type implies both its sides.
func (g *gen) checked(t ir.TypeRef, read, path string, big bool) string {
	literals, base := rangeSubject(t)
	if ir.RangeFailure(&base) == "" || ir.Implied(&base, false) && ir.Implied(&base, true) {
		return read
	}
	big = big && base.Kind == types.Int
	lo, hi, inclusive := rangeSides(base, big)
	args := []string{read, path, lo, hi, strconv.FormatBool(inclusive), quote(ir.RangeText(&base))}
	if len(literals) > 0 {
		args = append(args, literalList(literals))
	}
	return g.call(rangeHelper(base, big), args...)
}

// rangeSubject is the type whose refinement is checked and the literals that pass it: a literal union's base and its literals, else t itself.
func rangeSubject(t ir.TypeRef) (literals []string, base ir.TypeRef) {
	if t.Kind == types.LitUnion && t.Elem != nil {
		return t.Literals, *t.Elem
	}
	return nil, t
}

// rangeHelper is the helper unit that holds a value of the level t to its refinement.
func rangeHelper(t ir.TypeRef, big bool) string {
	if h := rangeHelpers[t.Kind]; h != "" {
		return h
	}
	if big {
		return decInBigName
	}
	return decInName
}

// rangeSides are the lower and upper bounds a check compares and whether the upper one is inclusive: a side the stored type implies, or the refinement lacks, is open, ±Infinity (null for a bigint).
func rangeSides(t ir.TypeRef, big bool) (lo, hi string, inclusive bool) {
	r := t.Range
	lo, hi, inclusive = tsNegInfinity, tsInfinity, true
	if big {
		lo, hi = tsNull, tsNull
	}
	if r.HasLo && !ir.Implied(&t, false) {
		lo = limitText(t, r.Lo, big)
	}
	if r.HasHi && !ir.Implied(&t, true) {
		hi, inclusive = limitText(t, r.Hi, big), r.HiIncluded
	}
	return lo, hi, inclusive
}

// limitText is one bound of t as a literal: a binary64 for a Float, a bigint at a bigint position, else a number.
func limitText(t ir.TypeRef, l types.Limit, big bool) string {
	switch {
	case t.Kind == types.Float:
		return floatText(l.F)
	case big:
		return bigText(l.I)
	}
	return intText(l.I)
}

// literalList is the array of a union's literals, quoted.
func literalList(literals []string) string {
	quoted := make([]string, len(literals))
	for i, lit := range literals {
		quoted[i] = quote(lit)
	}
	return lbracket + joinArgs(quoted) + rbracket
}
