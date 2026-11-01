package gogen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// boundConsts writes, right after the struct of b, one const block of the finite sides of its fields' refinements (CODEGEN.md §5.15, DECISIONS 400).
func (g *gen) boundConsts(b *body) {
	var lines strings.Builder
	for _, f := range b.fields {
		for _, bc := range ir.FieldBounds(f) {
			name := g.names.BoundName(b.goName, f, bc)
			fmt.Fprintf(&lines, boundDocFormat, name, ir.BoundSideText(bc.Side), ir.BoundSubject(f, bc), ir.RangeText(bc.Level))
			typ, val := g.boundValue(bc)
			fmt.Fprintf(&lines, boundDeclFormat, name, typ, val)
		}
	}
	if lines.Len() > 0 {
		g.printf(boundBlockFormat, lines.String())
	}
}

// boundValue is the type to declare a bound constant with (empty for an untyped one) and its value: an Int bound untyped, a Float bound a float64, a Duration bound N * time.Millisecond (CODEGEN.md §5.15).
func (g *gen) boundValue(b ir.BoundConst) (typ, val string) {
	if b.Kind == types.Float {
		return space + goFloat64, floatBound(b.Limit.F)
	}
	if b.Kind == types.Duration {
		return "", g.durationLit(b.Limit.I)
	}
	return "", intLit(b.Limit.I)
}
