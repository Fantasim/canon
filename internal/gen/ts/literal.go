package tsgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// owner is the record whose field is being written: a dependent value reads its discriminant from it (CODEGEN.md §5.6).
type owner struct {
	fields []*ir.Field
	rec    *value.Record
}

// as is v as a *T; a value of another kind is an IR defect, reported once.
func as[T any](g *gen, v value.Value) *T {
	x, ok := any(v).(*T)
	if !ok || x == nil {
		g.failf(ErrMalformed, malformedValue, v, g.at, fmt.Sprintf("%T", x))
		return new(T)
	}
	return x
}

// lit is the TypeScript expression of v as tsType(t) holds it; big writes integers as bigint (CODEGEN.md §4.1).
func (g *gen) lit(t ir.TypeRef, v value.Value, big bool) string {
	big = bigAt(t, big)
	switch t.Kind {
	case types.Bool:
		return strconv.FormatBool(as[value.Bool](g, v).V)
	case types.Int:
		return g.intLit(as[value.Int](g, v).V, big)
	case types.Float:
		return floatText(as[value.Float](g, v).V)
	case types.String:
		return quote(as[value.Str](g, v).V)
	case types.LitUnion:
		return g.unionLit(t, v)
	case types.Duration:
		return intText(as[value.Dur](g, v).Ms)
	case types.Enum:
		return quote(g.memberWire(t.Named, as[value.Member](g, v).Index))
	case types.VariantKind:
		return quote(g.caseWire(t.Named, as[value.CaseKind](g, v).Index))
	case types.Ref:
		return g.keyLit(t, as[value.Ref](g, v).Key, big)
	case types.Optional:
		return g.optionalLit(t, v, big)
	default:
		return g.compositeLit(t, v, big)
	}
}

// unionLit is a literal union's value: its literal, or the base type's value (a member of the enum arm) as its wire text (CODEGEN.md §4.1).
func (g *gen) unionLit(t ir.TypeRef, v value.Value) string {
	if m, ok := v.(*value.Member); ok {
		return quote(g.memberWire(g.elem(t).Named, m.Index))
	}
	return quote(as[value.Str](g, v).V)
}

// compositeLit is the literal of a record, variant, list, table or map.
func (g *gen) compositeLit(t ir.TypeRef, v value.Value, big bool) string {
	switch t.Kind {
	case types.Record:
		return g.recordLit(g.recordOf(t.Named), as[value.Record](g, v))
	case types.Variant, types.Case:
		return g.variantLit(t, as[value.Record](g, v))
	case types.List:
		return g.listLit(t, as[value.List](g, v).Elems, big)
	case types.Table:
		return g.tableLit(t, as[value.Table](g, v))
	case types.Map, types.DepMap:
		return g.mapLit(t, as[value.Map](g, v), big)
	default:
		g.failf(errUnsupported, unsupportedKind, kindText(t.Kind), g.at)
		return tsUndefined
	}
}

// intLit is an integer: a bigint literal for a @ts(bigint) field, else a number.
func (g *gen) intLit(n int64, big bool) string {
	if big {
		return bigText(n)
	}
	return intText(n)
}

// optionalLit is null for none, else the element.
func (g *gen) optionalLit(t ir.TypeRef, v value.Value, big bool) string {
	if _, none := v.(*value.None); none {
		return tsNull
	}
	return g.lit(g.elem(t), v, big)
}

// recordOf is a named type as a record.
func (g *gen) recordOf(t ir.Type) *ir.Record {
	r, ok := t.(*ir.Record)
	if !ok {
		g.failf(ErrMalformed, malformedDecl, g.at)
		return &ir.Record{}
	}
	return r
}

func (g *gen) memberWire(named ir.Type, index int) string {
	e, ok := named.(*ir.Enum)
	if !ok || index < 0 || index >= len(e.Members) {
		g.failf(ErrMalformed, malformedMember, index, g.at)
		return ""
	}
	return e.Members[index].Wire
}

// memberNamed is the wire of the member of enum named called name.
func (g *gen) memberNamed(named ir.Type, name string) string {
	if e, ok := named.(*ir.Enum); ok {
		for _, m := range e.Members {
			if m.Name == name {
				return m.Wire
			}
		}
	}
	g.failf(ErrMalformed, malformedMemberName, name, g.at)
	return ""
}

func (g *gen) caseWire(named ir.Type, index int) string {
	v, ok := named.(*ir.Variant)
	if !ok || index < 0 || index >= len(v.Cases) {
		g.failf(ErrMalformed, malformedCase, index, g.at)
		return ""
	}
	return v.Cases[index].Wire
}

// keyLit is a ref's key: a string, or an integer key as bigAt decided for the ref (CODEGEN.md §5.8).
func (g *gen) keyLit(t ir.TypeRef, k value.Key, big bool) string {
	switch {
	case t.Key != nil && t.Key.Kind == types.Enum && !k.IsInt:
		return quote(g.memberNamed(t.Key.Named, k.S))
	case k.IsInt:
		return g.intLit(k.I, big)
	}
	return quote(k.S)
}

// listLit is `[a, b]`; a freezing wrapper is the caller's.
func (g *gen) listLit(t ir.TypeRef, elems []value.Value, big bool) string {
	et := g.elem(t)
	items := make([]string, len(elems))
	for i, x := range elems {
		items[i] = g.lit(et, x, big)
	}
	return lbracket + strings.Join(items, listSep) + rbracket
}

// tableLit is a nested table: the rows in entry order.
func (g *gen) tableLit(t ir.TypeRef, tab *value.Table) string {
	rec := g.recordOf(g.elem(t).Named)
	items := make([]string, len(tab.Entries))
	for i, r := range tab.Entries {
		items[i] = g.rowLit(rec, r)
	}
	return lbracket + strings.Join(items, listSep) + rbracket
}

// mapLit is `new CanonMap<K, V>([[k, v], ...])`: its writes throw (CODEGEN.md §8.1); composite values are frozen one by one, since a frozen map does not freeze them.
func (g *gen) mapLit(t ir.TypeRef, m *value.Map, big bool) string {
	if t.Key == nil {
		g.failf(ErrMalformed, malformedNoKey, g.at)
		return tsUndefined
	}
	kt, vt := *t.Key, g.elem(t)
	items := make([]string, len(m.Keys))
	for i := range m.Keys {
		items[i] = fmt.Sprintf(mapEntryFormat, g.lit(kt, m.Keys[i], big), g.freeze(vt, g.lit(vt, m.Vals[i], big), big))
	}
	g.helper(canonMapName)
	return fmt.Sprintf(newMapFormat, g.keyType(kt, big), g.tsType(vt, big), strings.Join(items, listSep))
}

// freeze wraps the literal of a composite value in canonFreeze, typed so that its strings stay literals.
func (g *gen) freeze(t ir.TypeRef, text string, big bool) string {
	switch t.Kind {
	case types.Record, types.Variant, types.Case, types.List, types.Table:
		return fmt.Sprintf(freezeFormat, g.helper(canonFreezeName), g.tsType(t, big), text)
	default:
		return text
	}
}
