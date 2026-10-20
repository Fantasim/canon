package gogen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// optionalType is an optional list element or map value: `*T`, T the element's own Go type, so `[R?]` has `[R]`'s type and only nil differs (CODEGEN.md §4.2; DECISIONS 385).
func (g *gen) optionalType(t ir.TypeRef) string {
	elem := g.sub(t.Elem)
	typ := g.goType(elem)
	if ir.GoPointerKind(elem.Kind) {
		return typ
	}
	return pointer + typ
}

// optionalExpr is an optional element's value: nil for none, a record or variant as in `[R]`, else rt.Ptr of the element's literal, typed so an untyped constant keeps its Go type (CODEGEN.md §4.2; DECISIONS 385).
func (g *gen) optionalExpr(t ir.TypeRef, v value.Value) string {
	if _, none := v.(*value.None); none {
		return nilLit
	}
	elem := g.sub(t.Elem)
	x := g.expr(elem, v)
	if ir.GoPointerKind(elem.Kind) {
		return x
	}
	return fmt.Sprintf(ptrFormat, g.rt(), g.goType(elem), x)
}

// readOptional reads an optional element or map value: JSON null is none (nil), anything else its element (WIRE.md §5.4, CODEGEN.md §4.2; DECISIONS 385).
func (g *gen) readOptional(b *strings.Builder, l leaf, raw string, loc location) string {
	elem := g.sub(l.t.Elem)
	v := g.temp(tempValue)
	fmt.Fprintf(b, optionalOpenFormat, v, g.optionalType(l.t), raw)
	x := g.readValue(b, leaf{t: elem, unit: l.unit, enc: l.enc, disc: l.disc, owned: l.owned, table: l.table}, raw, loc)
	if !ir.GoPointerKind(elem.Kind) {
		x = fmt.Sprintf(ptrFormat, g.rt(), g.goType(elem), x)
	}
	fmt.Fprintf(b, cellStoreFormat, v, x)
	b.WriteString(closeBrace)
	return v
}

// walkOptional walks an optional element or map value: a record or variant is its nilable pointer, any other element is dereferenced once present.
func (g *gen) walkOptional(b *strings.Builder, t ir.TypeRef, expr string, loc location, inline bool) {
	elem := g.sub(t.Elem)
	if ir.GoPointerKind(elem.Kind) {
		g.walkValue(b, elem, expr, loc, inline)
		return
	}
	fmt.Fprintf(b, ifNotNilFormat, expr)
	g.walkValue(b, elem, fmt.Sprintf(derefFormat, expr), loc, inline)
	b.WriteString(closeBrace)
}
