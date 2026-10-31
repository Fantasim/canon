package cppgen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

var (
	// sideText is T19's <Side> by the side of a bound.
	sideText = [...]string{ir.SideMin: sideLower, ir.SideMax: sideUp, ir.SideEnd: sideExclusive}
	// stepText is T19's subject words by the step of a bound's path.
	stepText = [...]string{ir.StepElem: eachElem, ir.StepKey: eachKey, ir.StepValue: eachValue}
)

// boundConsts writes the bound constants of the fields a class writes, at the start of its public members (CODEGEN.md §5.15, DECISIONS 400).
func (g *gen) boundConsts(c class, fields []*ir.Field) {
	for _, f := range fields {
		if f.Optional && f.Type.Kind == types.Never || f.Input != nil && c.rec == nil {
			continue
		}
		for _, b := range ir.FieldBounds(f) {
			g.h.linef(1, boundDocFormat, sideText[b.Side], boundSubject(f, b), ir.RangeText(b.Level))
			g.h.linef(1, boundDeclFormat, g.boundType(b), g.pl.BoundName(f, b), g.boundValue(b))
		}
	}
}

// boundSubject is T19's subject: the field's Canon name, each step of the bound's path before it (the deepest first), a length's prefix before all.
func boundSubject(f *ir.Field, b ir.BoundConst) string {
	subject := f.Name
	for _, step := range b.Path {
		subject = stepText[step] + subject
	}
	switch {
	case b.Length && b.Level.Kind == types.String:
		return byteLengthOf + subject
	case b.Length:
		return countOf + subject
	}
	return subject
}

// boundType is the C++ type of a bound constant: int64_t, double or std::chrono::milliseconds (CODEGEN.md §5.15).
func (g *gen) boundType(b ir.BoundConst) string {
	switch b.Kind {
	case types.Float:
		return cppDouble
	case types.Duration:
		return cppMillis
	default:
		return cppInt64
	}
}

// boundValue is the folded bound as it is, never shifted or clamped.
func (g *gen) boundValue(b ir.BoundConst) string {
	switch b.Kind {
	case types.Float:
		return g.floatLit(b.Limit.F, bits64)
	case types.Duration:
		return fmt.Sprintf(boundMillisFormat, intLit(b.Limit.I))
	default:
		return intLit(b.Limit.I)
	}
}
