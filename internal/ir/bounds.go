package ir

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// BoundStep is one step of a bound constant's path below its field: StepElem, StepKey or StepValue.
type BoundStep uint8

// BoundSide is the side of a refinement a bound constant names: SideMin, SideMax or SideEnd.
type BoundSide uint8

// BoundConst is one finite side of the refinement of a level of a field's type, a constant of the field (CODEGEN.md §5.15, DECISIONS 400).
type BoundConst struct {
	Path   []BoundStep
	Length bool // a String's bytes or a collection's count
	Side   BoundSide
	Kind   types.Kind // Int, Float or Duration (Int for a length)
	Big    bool       // TS: an Int bound at a @ts(bigint) position
	Limit  types.Limit
	Level  *TypeRef // for <R>
}

// FieldBounds are f's bound constants (CODEGEN.md §5.15): depth first from the field's type, Key before Value, Min before Max or End; an optional and a literal union's base add no step, a named or dependent type ends the walk.
func FieldBounds(f *Field) []BoundConst {
	var out []BoundConst
	walkBounds(&f.Type, nil, f.BigInt, &out)
	return out
}

// walkBounds appends t's own bounds at path, then those of the levels below it.
func walkBounds(t *TypeRef, path []BoundStep, big bool, out *[]BoundConst) {
	switch {
	case t == nil:
	case t.Kind == types.Optional || t.Kind == types.LitUnion:
		walkBounds(t.Elem, path, big, out)
	case t.Kind == types.List:
		*out = append(*out, levelBounds(t, path, big)...)
		walkBounds(t.Elem, stepTo(path, StepElem), big, out)
	case t.Kind == types.Map || t.Kind == types.DepMap:
		*out = append(*out, levelBounds(t, path, big)...)
		walkBounds(t.Key, stepTo(path, StepKey), big, out)
		walkBounds(t.Elem, stepTo(path, StepValue), big, out)
	default:
		*out = append(*out, levelBounds(t, path, big)...)
	}
}

// stepTo is path followed by s, a new slice.
func stepTo(path []BoundStep, s BoundStep) []BoundStep { return append(slices.Clone(path), s) }

// levelBounds are the finite sides of t's own refinement, Min first.
func levelBounds(t *TypeRef, path []BoundStep, big bool) []BoundConst {
	if t.Range == nil || !rangeRefines(t.Kind) {
		return nil
	}
	b := BoundConst{Path: path, Length: rangeLength(t.Kind), Kind: t.Kind, Level: t}
	if b.Length {
		b.Kind = types.Int
	}
	b.Big = big && !b.Length && b.Kind == types.Int
	var out []BoundConst
	if t.Range.HasLo {
		lo := b
		lo.Side, lo.Limit = SideMin, t.Range.Lo
		out = append(out, lo)
	}
	if t.Range.HasHi {
		hi := b
		hi.Side, hi.Limit = SideEnd, t.Range.Hi
		if t.Range.HiIncluded {
			hi.Side = SideMax
		}
		out = append(out, hi)
	}
	return out
}

// boundSuffix is what follows <F> in a bound constant's name: the path's words, Len for a length, the side (CODEGEN.md §5.15).
func boundSuffix(b BoundConst) string {
	var s strings.Builder
	for _, st := range b.Path {
		s.WriteString(boundStepWords[st])
	}
	if b.Length {
		s.WriteString(GoLen)
	}
	s.WriteString(boundSideWords[b.Side])
	return s.String()
}

// rangeFailureOf is the failure prefix of a refinement of kind k, "" for a kind no range refines.
func rangeFailureOf(k types.Kind) string {
	if int(k) < len(rangeFailures) {
		return rangeFailures[k]
	}
	return ""
}

// rangeRefines reports a kind a range refines (TYPES.md §7.4).
func rangeRefines(k types.Kind) bool { return rangeFailureOf(k) != "" }

// rangeLength reports a kind whose range is a length: a String's bytes, a collection's count.
func rangeLength(k types.Kind) bool { return rangeRefines(k) && rangeFailureOf(k) != failValue }

// RangeText is t's refinement in Canon's text, "" for none (CODEGEN.md §5.15 `<R>`): `1..=120`, `0..1`, `..=8`, `1s..=1h`.
func RangeText(t *TypeRef) string {
	if t == nil || t.Range == nil {
		return ""
	}
	r, s := t.Range, ""
	if r.HasLo {
		s = limitText(r.Lo, t.Kind)
	}
	switch {
	case !r.HasHi:
		return s + rangeExclusive
	case r.HiIncluded:
		return s + rangeInclusive + limitText(r.Hi, t.Kind)
	}
	return s + rangeExclusive + limitText(r.Hi, t.Kind)
}

// limitText is one bound: a binary64 float (also of Float32, TYPES.md §7.3), a Duration literal, else a decimal integer.
func limitText(l types.Limit, k types.Kind) string {
	switch k {
	case types.Float:
		return types.FloatText(l.F, rangeFloatBits)
	case types.Duration:
		return types.DurationText(l.I)
	default:
		return strconv.FormatInt(l.I, decimalBase)
	}
}

// RangeFailure is the decoder's failure for a value outside t's refinement, "" for none (CODEGEN.md §5.15): a value, a byte length or an element count, then <R>.
func RangeFailure(t *TypeRef) string {
	if t == nil || t.Range == nil || !rangeRefines(t.Kind) {
		return ""
	}
	return rangeFailureOf(t.Kind) + RangeText(t)
}

// Implied reports a side of t's refinement that writes no comparison (CODEGEN.md §5.15): an absent side, or an integer, Duration or length side the stored type already guarantees; a Float side always compares.
func Implied(t *TypeRef, upper bool) bool {
	if t == nil || t.Range == nil {
		return true
	}
	r := t.Range
	if upper && !r.HasHi || !upper && !r.HasLo {
		return true
	}
	lo, hi, ok := storedLimits(t)
	switch {
	case !ok:
		return false
	case !upper:
		return r.Lo.I <= lo
	case r.HiIncluded:
		return r.Hi.I >= hi
	}
	return r.Hi.I > hi
}

// storedLimits are the values t's stored type holds: a length's, its width's, a Duration's; false for a Float and for an integer without a width (a hand-built TypeRef), whose every side compares.
func storedLimits(t *TypeRef) (lo, hi int64, ok bool) {
	if rangeLength(t.Kind) {
		return 0, math.MaxInt64, true
	}
	if t.Kind == types.Int && t.Bits <= 0 {
		return 0, 0, false
	}
	return types.Basic{K: t.Kind, Bits: t.Bits, Signed: t.Signed}.Limits()
}
