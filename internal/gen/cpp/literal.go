package cppgen

import (
	_ "embed"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

//go:embed text/schema_doc.txt
var schemaDocText string

// quote is a C++ string literal of s's bytes (the escapes table, other control bytes in octal).
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		esc, escaped := escapes[c]
		switch {
		case escaped:
			b.WriteString(esc)
		case c < ' ' || c == asciiDel:
			fmt.Fprintf(&b, octalEscapeFormat, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// intLit is an integer literal; the int64 limits are the runtime's constants (CONFORMANCE.md §7.2).
func intLit(v int64) string {
	switch v {
	case math.MinInt64:
		return intMinConst
	case math.MaxInt64:
		return intMaxConst
	}
	return strconv.FormatInt(v, decimalBase)
}

// floatLit is ECMAScript's shortest text, `.0` on an integral value, `-0.0` and `f` (CONFORMANCE.md §5).
func (g *gen) floatLit(f float64, bits int) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		g.fail(fmt.Errorf("%w: non-finite float at %s", ErrMalformed, g.at))
		return cppInvalid
	}
	text := types.FloatText(f, bits)
	switch {
	case f == 0 && math.Signbit(f):
		text = negZero
	case !strings.ContainsAny(text, floatMarks):
		text += pointZero
	}
	if bits == float32Bits {
		text += float32Suffix
	}
	return text
}

// literal is the C++ expression of v as a member or pure parameter of type t holds it; a
// Duration is its millisecond count, an enum member its enumerator, a ref its key.
func (g *gen) literal(t ir.TypeRef, v value.Value) string {
	switch {
	case t.Kind == types.LitUnion:
		return g.unionLit(t, v)
	case t.Kind == types.Optional && t.Elem != nil && !absent(v): // an optional element or map value (DECISIONS 385)
		return g.element(*t.Elem, v)
	}
	switch x := v.(type) {
	case *value.Bool:
		return strconv.FormatBool(x.V)
	case *value.Int:
		return intLit(x.V)
	case *value.Float:
		return g.floatLit(x.V, t.Bits)
	case *value.Str:
		return g.strLit(x.V)
	case *value.Dur:
		return intLit(x.Ms)
	case *value.Member:
		return g.memberLit(t, x.Index)
	case *value.Ref:
		return g.refLit(t, x.Key)
	case *value.None:
		return cppNullopt
	case *value.List:
		return g.listLit(t, x)
	case *value.Map:
		return g.mapLit(t, x)
	case *value.Table:
		return g.tableLit(t, x)
	case *value.Record:
		g.malformed(fmt.Sprintf(valueFormat, v), g.at) // E8019 RecordConstant: only a constant writes a record literal
		return cppInvalid
	}
	g.malformed(fmt.Sprintf(valueFormat, v), g.at) // a value of a kind refuseKind meets first
	return cppInvalid
}

// unionLit is a literal union's value as its storage holds it, the wire text: a literal, a member of its enum, or its ref's key (CODEGEN.md §4.1; DECISIONS 387).
func (g *gen) unionLit(t ir.TypeRef, v value.Value) string {
	var e *ir.Enum
	if t.Elem != nil {
		e, _ = t.Elem.Named.(*ir.Enum)
	}
	switch x := v.(type) {
	case *value.Str:
		return g.strLit(x.V)
	case *value.Member:
		if e != nil && x.Index >= 0 && x.Index < len(e.Members) {
			return g.strLit(e.Members[x.Index].Wire)
		}
	case *value.Ref:
		if r := ir.UnionRef(t); r != nil {
			return g.strLit(ir.KeyWire(*r, x.Key))
		}
	}
	g.malformed(fmt.Sprintf(valueFormat, v), g.at)
	return cppInvalid
}

// keyMember is the enum keying ref r and the index of key k's member in it; -1 for a key of no enum or naming no member.
func keyMember(r ir.TypeRef, k value.Key) (*ir.Enum, int) {
	e, ok := ir.RefKeyEnum(r)
	if !ok {
		return nil, -1
	}
	return e, ir.MemberIndex(e, k.S)
}

// refLit is a ref's key as its storage holds it: a table id's or an enum key's member, by name, else the key itself (CODEGEN.md §5.3, §5.8).
func (g *gen) refLit(t ir.TypeRef, k value.Key) string {
	if id, ok := g.idEnum(t); ok {
		return id + scopeSep + g.pl.IDMember(k.S)
	}
	if _, i := keyMember(t, k); i >= 0 {
		return g.memberLit(*t.Key, i)
	}
	return g.keyLit(k)
}

func (g *gen) keyLit(k value.Key) string {
	if k.IsInt {
		return intLit(k.I)
	}
	return g.strLit(k.S)
}

// strLit is a string value's literal, sized when it holds a NUL a literal's own length stops at (CODEGEN.md §3.4).
func (g *gen) strLit(s string) string {
	switch g.strs {
	case viewForm:
		return viewLit(s)
	case arrayForm:
		return fmt.Sprintf(viewFormat, quote(s), len(s))
	default:
		return stringLit(s)
	}
}

// stringLit is s as a literal a std::string is built from at its full length.
func stringLit(s string) string {
	if !strings.Contains(s, nulText) {
		return quote(s)
	}
	return fmt.Sprintf(sizedStringFormat, quote(s), len(s))
}

// viewLit is s as a literal a std::string_view compares or holds at its full length.
func viewLit(s string) string {
	if !strings.Contains(s, nulText) {
		return quote(s)
	}
	return fmt.Sprintf(viewFormat, quote(s), len(s))
}

// inForm is text written with strings in form f: viewForm where a string is a std::string_view (a
// constexpr constant or cell, a conformance vector), arrayForm in a constexpr array, every string
// sized from its raw length, which no compiler then measures in constant evaluation.
func (g *gen) inForm(f strForm, text func() string) string {
	prev := g.strs
	g.strs = f
	defer func() { g.strs = prev }()
	return text()
}

// inViews is text written in viewForm.
func (g *gen) inViews(text func() string) string { return g.inForm(viewForm, text) }

// arrayText is text written in arrayForm.
func (g *gen) arrayText(text func() string) string { return g.inForm(arrayForm, text) }

func (g *gen) memberLit(t ir.TypeRef, index int) string {
	e, ok := t.Named.(*ir.Enum)
	if !ok || index < 0 || index >= len(e.Members) {
		g.fail(fmt.Errorf("%w: enum member %d at %s", ErrMalformed, index, g.at))
		return cppInvalid
	}
	return g.typeName(e) + scopeSep + g.pl.Enumerator(e.Members[index])
}

// element is the literal of one element or map entry part; a Duration is written as one.
func (g *gen) element(t ir.TypeRef, v value.Value) string {
	text := g.literal(t, v)
	if t.Kind == types.Duration {
		return cppMillis + openBrace + text + closeBrace
	}
	return text
}

func (g *gen) listLit(t ir.TypeRef, l *value.List) string {
	if t.Elem == nil {
		g.fail(fmt.Errorf("%w: list without an element type at %s", ErrMalformed, g.at))
		return cppInvalid
	}
	items := make([]string, len(l.Elems))
	for i, e := range l.Elems {
		items[i] = g.element(*t.Elem, e)
	}
	return openBrace + strings.Join(items, listSep) + closeBrace
}

// mapLit builds a FlatMap from its entries in insertion order (CODEGEN.md §4.2, §5.1).
func (g *gen) mapLit(t ir.TypeRef, m *value.Map) string {
	if t.Key == nil || t.Elem == nil || len(m.Keys) != len(m.Vals) {
		g.fail(fmt.Errorf("%w: map without its key or value type at %s", ErrMalformed, g.at))
		return cppInvalid
	}
	items := make([]string, len(m.Keys))
	for i, k := range m.Keys {
		items[i] = openBrace + g.element(*t.Key, k) + listSep + g.element(*t.Elem, m.Vals[i]) + closeBrace
	}
	return fmt.Sprintf(fromEntriesFormat, g.storage(t), strings.Join(items, listSep))
}

// constants writes all but enum-typed constants, which follow the enums (log-2026-09-24, gen/cpp).
func (g *gen) constants() {
	for _, c := range g.p.Consts {
		if !enumTyped(c.Type) {
			g.constant(c)
		}
	}
}

func (g *gen) enumConstants() {
	for _, c := range g.p.Consts {
		if enumTyped(c.Type) {
			g.constant(c)
		}
	}
}

func enumTyped(t ir.TypeRef) bool {
	switch t.Kind {
	case types.Enum:
		return true
	case types.List, types.Map, types.Optional:
		return t.Elem != nil && enumTyped(*t.Elem) || t.Key != nil && enumTyped(*t.Key)
	default:
		return false
	}
}

// constant is one `inline constexpr` (`inline const` for a list or map) declaration (§5.1).
func (g *gen) constant(c *ir.Const) {
	leave := g.enter(c.Name)
	defer leave()
	name := g.pl.ConstName(c)
	var list string
	if c.Type.Kind == types.List || c.Type.Kind == types.Map {
		list = g.constListLit(c) // its array, if any, precedes the doc
	}
	g.doc(0, c.Doc)
	switch c.Type.Kind {
	case types.Enum:
		format := constexprFormat
		if e, ok := c.Type.Named.(*ir.Enum); ok && g.open(e) { // a class holding a std::string is no constant expression
			format = listConstFormat
		}
		g.h.printf(format, g.storage(c.Type), name, g.literal(c.Type, c.V))
	case types.Bool, types.Int, types.Float:
		g.h.printf(constexprFormat, g.storage(c.Type), name, g.literal(c.Type, c.V))
	case types.String, types.LitUnion:
		g.h.printf(constexprFormat, cppStringView, name, g.inViews(func() string { return g.literal(c.Type, c.V) }))
	case types.Duration:
		g.h.printf(durationConstFormat, cppMillis, name, g.literal(c.Type, c.V))
	case types.List, types.Map:
		g.h.printf(listConstFormat, g.storage(c.Type), name, list)
	default:
		g.refuseKind(c.Type.Kind, c.Name)
	}
	g.h.blank()
}

// schemaConstants are k<V>Schema, one per emitted value, with T2 (CODEGEN.md §2.5, §3.3).
func (g *gen) schemaConstants() {
	if g.baked() {
		return // a baked emit reads no file (decision 121)
	}
	for _, v := range g.values {
		if v.Schema == "" {
			g.fail(fmt.Errorf("%w: value %s without a schema", ErrMalformed, v.Name))
			continue
		}
		g.h.printf(schemaDocText, dataFile(v))
		g.h.printf(constexprFormat, cppStringView, g.pl.SchemaName(v), quote(v.Schema))
		g.h.blank()
	}
}

// constListLit is a list or map constant's initializer: a long one built from an array of the
// header's detail namespace, k + its name + Elems, so that no TU's initializer grows with it
// (DECISIONS 392).
func (g *gen) constListLit(c *ir.Const) string {
	prev := g.pool
	defer func() { g.pool = prev }()
	g.pool = &bulk{open: inlineArrayOpenFormat, prefix: detailPrefix, name: func(n int) string { return g.pl.ElemsName(c, n) }}
	text, ok := g.hoistedLiteral(c.Type, c.V)
	if !ok {
		return g.literal(c.Type, c.V)
	}
	g.h.line(detailOpen)
	g.h.blank()
	g.h.write(g.pool.w.String())
	g.h.line(detailClose)
	g.h.blank()
	return text
}
