package gogen

import (
	"math"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// expr is the Go expression of v as goType(t) holds it. Refs are keys (CODEGEN.md §5.8).
func (g *gen) expr(t ir.TypeRef, v value.Value) string {
	switch t.Kind {
	case types.Bool:
		return strconv.FormatBool(as[value.Bool](g, v).V)
	case types.Int:
		return strconv.FormatInt(as[value.Int](g, v).V, decimal)
	case types.Float:
		return g.floatLit(as[value.Float](g, v).V, t.Bits)
	case types.String, types.LitUnion:
		return strconv.Quote(as[value.Str](g, v).V)
	case types.Duration:
		return g.durationLit(as[value.Dur](g, v).Ms)
	case types.Enum:
		return g.memberLit(t, as[value.Member](g, v).Index)
	case types.VariantKind:
		return g.kindLit(t.Named, as[value.CaseKind](g, v).Index)
	case types.Record:
		return g.recordExpr(t, as[value.Record](g, v))
	case types.Variant, types.Case:
		return g.variantExpr(t, as[value.Record](g, v))
	case types.List:
		return g.listExpr(t, as[value.List](g, v))
	case types.Map, types.DepMap:
		return g.mapExpr(t, as[value.Map](g, v))
	case types.Ref:
		return g.keyLit(t, as[value.Ref](g, v).Key)
	default:
		g.failKind(t.Kind)
		return ""
	}
}

// as is v as a *T; a value of another kind is an IR defect, reported once.
func as[T any](g *gen, v value.Value) *T {
	x, ok := any(v).(*T)
	if !ok || x == nil {
		g.failf(ErrMalformed, "a %T value where a %T is expected", v, x)
		return new(T)
	}
	return x
}

// floatLit is the shortest literal that reads back at its width; -0.0 is Copysign (decision 181).
func (g *gen) floatLit(f float64, bits int) string {
	if f == 0 && math.Signbit(f) {
		z := g.use(mathPkg, mathPkg) + negativeZero
		if bits == float32Bits {
			return g.sized(floatTypes, bits) + lparen + z + rparen
		}
		return z
	}
	return strconv.FormatFloat(f, floatFormat, shortest, bits)
}

// durationLit is always a product with time.Millisecond, so a constant keeps its type.
func (g *gen) durationLit(ms int64) string {
	return strconv.FormatInt(ms, decimal) + timesMillisecond + g.use(timePkg, timePkg) + millisecond
}

func (g *gen) memberLit(t ir.TypeRef, index int) string {
	e, ok := t.Named.(*ir.Enum)
	if !ok || index < 0 || index >= len(e.Members) {
		g.failf(ErrMalformed, "member %d of enum %s", index, qname(t.Named))
		return zeroLit
	}
	return g.qualify(e.Pkg, memberName(g.goName(e), e.Members[index]))
}

func memberName(enum string, m *ir.EnumMember) string {
	if m.Go.Name != "" {
		return m.Go.Name
	}
	return enum + upperCamel(m.Name)
}

func (g *gen) kindLit(named ir.Type, index int) string {
	v, ok := named.(*ir.Variant)
	if !ok || index < 0 || index >= len(v.Cases) {
		g.failf(ErrMalformed, "case %d of variant %s", index, qname(named))
		return zeroLit
	}
	kind := kindName(g.goName(v))
	c := v.Cases[index]
	return g.qualify(v.Pkg, kind+exportedName(c.Go.Name, c.Name))
}

// keyLit is a ref's key: a table id constant, or a literal of the key's type (§5.8).
func (g *gen) keyLit(t ir.TypeRef, k value.Key) string {
	if isTableRef(t.Ref) {
		return g.qualify(t.Ref.Pkg, idMemberName(idTypeName(g.goName(t.Ref.Elem)), k.S))
	}
	switch {
	case t.Key == nil:
		g.failf(ErrMalformed, noKeyType)
	case t.Key.Kind == types.Enum:
		return g.memberLit(*t.Key, memberIndex(t.Key.Named, k.S))
	case k.IsInt:
		return strconv.FormatInt(k.I, decimal)
	}
	return strconv.Quote(k.S)
}

func idMemberName(idType, key string) string { return idType + upperCamel(key) }

// memberIndex is the index of the enum member named name, or -1.
func memberIndex(named ir.Type, name string) int {
	if e, ok := named.(*ir.Enum); ok {
		for i, m := range e.Members {
			if m.Name == name {
				return i
			}
		}
	}
	return -1
}

// listExpr is an rt.List, or an rt.KeyedList of rows and their keys.
func (g *gen) listExpr(t ir.TypeRef, v *value.List) string {
	if t.KeyedBy != nil {
		return g.keyedListExpr(t, v)
	}
	elem := g.goType(g.sub(t.Elem))
	if len(v.Elems) == 0 {
		return g.rt() + listType + lbracket + elem + rbracket + emptyBraces
	}
	items := make([]string, len(v.Elems))
	for i, x := range v.Elems {
		items[i] = g.expr(g.sub(t.Elem), x)
	}
	return g.rt() + makeList + sliceOf + elem + braced(elide(elem, items)) + rparen
}

func (g *gen) keyedListExpr(t ir.TypeRef, v *value.List) string {
	key := g.keyField(t)
	rec, ok := g.sub(t.Elem).Named.(*ir.Record)
	if !ok {
		return nilLit
	}
	rows := make([]string, len(v.Elems))
	keys := make([]string, len(v.Elems))
	for i, x := range v.Elems {
		r := as[value.Record](g, x)
		rows[i] = g.recordLit(rec, r)
		keys[i] = g.expr(key.Type, g.fieldValue(r, key.Name))
	}
	return g.rt() + makeKeyedList + sliceOf + g.typeName(rec) + braced(elide(g.typeName(rec), rows)) + listSep +
		sliceOf + g.goType(key.Type) + braced(keys) + rparen
}

func (g *gen) mapExpr(t ir.TypeRef, v *value.Map) string {
	kt, vt := g.goType(g.sub(t.Key)), g.goType(g.sub(t.Elem))
	if len(v.Keys) == 0 {
		return g.rt() + mapType + lbracket + kt + listSep + vt + rbracket + emptyBraces
	}
	keys := make([]string, len(v.Keys))
	vals := make([]string, len(v.Vals))
	for i := range v.Keys {
		keys[i] = g.expr(g.sub(t.Key), v.Keys[i])
		vals[i] = g.expr(g.sub(t.Elem), v.Vals[i])
	}
	return g.rt() + makeMap + sliceOf + kt + braced(elide(kt, keys)) + listSep + sliceOf + vt + braced(elide(vt, vals)) + rparen
}

// elide drops the type of each item that is a composite literal of typ, or &T{…} for typ *T:
// the element type of the slice or array that holds them already says it (gofmt -s).
func elide(typ string, items []string) []string {
	addr := ampersand + strings.TrimPrefix(typ, pointer) + lbrace
	for i, x := range items {
		if strings.HasPrefix(x, typ+lbrace) || (strings.HasPrefix(typ, pointer) && strings.HasPrefix(x, addr)) {
			items[i] = x[len(typ):]
		}
	}
	return items
}

// braced is {a, b} on one line, or one item per line when an item spans lines or the line
// would pass maxInline bytes.
func braced(items []string) string {
	inline := strings.Join(items, listSep)
	if len(inline) > maxInline || strings.Contains(inline, newline) {
		return lbrace + newline + strings.Join(items, listEnd) + listEnd + rbrace
	}
	return lbrace + inline + rbrace
}

func qname(t ir.Type) string {
	if t == nil {
		return ""
	}
	return t.QName()
}
