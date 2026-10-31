package tsgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

var (
	// sideText is T19's <side> by the side of a bound.
	sideText = [...]string{ir.SideMin: lowerWord, ir.SideMax: upperWord, ir.SideEnd: exclusiveWord}
	// stepText is T19's step word by the step of a bound's path.
	stepText = [...]string{ir.StepElem: stepElem, ir.StepKey: stepKey, ir.StepValue: stepValue}
)

// boundConsts writes, right after the interface of the record or case owner, the constants of the finite sides of its fields' refinements (CODEGEN.md §5.15, DECISIONS 400); a name stays reserved when its bound is not written.
func (g *gen) boundConsts(owner string, fields []*ir.Field) {
	var lines strings.Builder
	for _, f := range fields {
		if f.Input != nil {
			continue
		}
		for _, b := range ir.FieldBounds(f) {
			typ, val, ok := boundValue(b)
			if !ok {
				continue
			}
			name := g.declare(ir.TSBoundName(owner, f, b), g.at+dot+f.Name)
			fmt.Fprintf(&lines, boundDocFormat, sideText[b.Side], boundSubject(f, b), ir.RangeText(b.Level))
			fmt.Fprintf(&lines, typedConstFormat, name, typ, val)
		}
	}
	g.add(lines.String())
}

// boundSubject is T19's subject: the field's Canon name, each step of the bound's path before it (the deepest first), a length's prefix before all.
func boundSubject(f *ir.Field, b ir.BoundConst) string {
	subject := f.Name
	for _, step := range b.Path {
		subject = stepText[step] + space + subject
	}
	switch {
	case b.Length && b.Level.Kind == types.String:
		return byteLength + space + subject
	case b.Length:
		return elemCount + space + subject
	}
	return subject
}

// boundValue is the type and the literal of a bound constant: a bigint at a @ts(bigint) position, a number otherwise (a Duration in milliseconds); not written (false) for an integer past ±(2^53 − 1) at a number position (CODEGEN.md §5.15).
func boundValue(b ir.BoundConst) (typ, val string, ok bool) {
	switch {
	case b.Kind == types.Float:
		return tsNumber, floatText(b.Limit.F), true
	case b.Big:
		return tsBigint, bigText(b.Limit.I), true
	}
	n := b.Limit.I
	return tsNumber, intText(n), n >= -maxSafeInt && n <= maxSafeInt
}
