package gogen

import (
	"slices"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// opaqueEnum reports an enum written as a struct holding its declaration position: one without @codes that no types-mode emit opens (CODEGEN.md §5.2, DECISIONS 402).
func (g *gen) opaqueEnum(t ir.Type) bool {
	e, ok := t.(*ir.Enum)
	return ok && e.Codes == nil && !ir.OpenEnum(g.p, g.e, e)
}

// opaqueRef reports a type that is such an enum.
func (g *gen) opaqueRef(t ir.TypeRef) bool {
	return t.Kind == types.Enum && g.opaqueEnum(t.Named)
}

// zeroNumber is the placeholder of an absent number or enum: an untyped 0, or the zero struct of an opaque enum (CODEGEN.md §5.2, DECISIONS 402).
func (g *gen) zeroNumber(t ir.TypeRef) string {
	if g.opaqueRef(t) {
		return g.goType(t) + emptyBraces
	}
	return zeroLit
}

// positionCompare reports an ordering comparison of an enum without @codes: its members compare by Position() (CONFORMANCE.md §3, DECISIONS 402).
func (g *gen) positionCompare(x *ir.Binary) bool {
	return slices.Contains(orderOps, x.Op) && g.opaqueRef(x.X.Type())
}

// orderOps are the ordering comparisons.
var orderOps = []ir.Op{ir.OpLt, ir.OpLe, ir.OpGt, ir.OpGe}

// positionOperand is the Position() of an operand; one that is not inert is parenthesized first.
func (g *gen) positionOperand(v string) string {
	if inert(v) {
		return positionOf(v)
	}
	return positionOf(lparen + v + rparen)
}

// enumConstFormat declares a Canon const of an enum: a var for an opaque enum, a const otherwise.
func (g *gen) enumConstFormat(t ir.TypeRef) string {
	if g.opaqueRef(t) {
		return varDeclFormat
	}
	return constDeclFormat
}

// positionOf is the array index of the opaque value x: its Position() (CODEGEN.md §5.2, T20).
func positionOf(x string) string { return x + dot + positionMethod + callSuffix }
