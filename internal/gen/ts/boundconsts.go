package tsgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
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
			fmt.Fprintf(&lines, boundDocFormat, ir.BoundSideText(b.Side), ir.BoundSubject(f, b), ir.RangeText(b.Level))
			fmt.Fprintf(&lines, typedConstFormat, name, typ, val)
		}
	}
	g.add(lines.String())
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
