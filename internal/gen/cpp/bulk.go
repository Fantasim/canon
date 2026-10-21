package cppgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// bulk holds the namespace-scope arrays long literals are hoisted into, so that no element
// becomes code in a function (DECISIONS 392): a baked Build's in the .gen.cpp, a list or map
// constant's in the header's detail namespace.
type bulk struct {
	w      writer
	n      int
	open   string           // an array's declaration, of its element type and name
	name   func(int) string // the n-th array's name
	prefix string           // what code outside the arrays' namespace names them with
}

// bakedBulk is the pool of a baked Build: constexpr arrays of the .gen.cpp, numbered in emission order.
func bakedBulk() *bulk {
	return &bulk{open: arrayOpenFormat, name: ir.CppBulkArray}
}

// array writes items as an array of elem and returns how code names it.
func (b *bulk) array(elem string, items []string) string {
	name := b.name(b.n)
	b.n++
	b.w.printf(b.open, elem, name)
	for _, it := range items {
		b.w.lineAt(1, it+comma)
	}
	b.w.line(closeClass)
	b.w.blank()
	return b.prefix + name
}

// hoists reports n elements a baked Build writes from an array: the pool is open and n reaches bulkMinElems.
func (g *gen) hoists(n int) bool { return g.pool != nil && n >= bulkMinElems }

// arrayElem is the constant-expression type an array holds an element of type t as, false for one
// no constexpr array can hold (an opened enum, a list, a map, a record): a string is its view.
func (g *gen) arrayElem(t ir.TypeRef) (string, bool) {
	switch t.Kind {
	case types.Bool, types.Int, types.Float, types.Duration:
		return g.storage(t), true
	case types.String, types.LitUnion:
		return cppStringView, true
	case types.Enum:
		e, ok := t.Named.(*ir.Enum)
		return g.storage(t), ok && !g.open(e)
	case types.Ref:
		if id, ok := g.idEnum(t); ok {
			return id, true
		}
		if t.Key == nil {
			return "", false
		}
		return g.arrayElem(*t.Key)
	case types.Optional:
		if t.Elem == nil {
			return "", false
		}
		elem, ok := g.arrayElem(*t.Elem)
		return fmt.Sprintf(optionalFormat, elem), ok
	default:
		return "", false
	}
}

// hoistElems writes vs, each an element of type t, as an array and returns its name; false
// when they stay inline: too few, or of a type arrayElem refuses.
func (g *gen) hoistElems(t ir.TypeRef, vs []value.Value) (string, bool) {
	if !g.hoists(len(vs)) {
		return "", false
	}
	elem, ok := g.arrayElem(t)
	if !ok {
		return "", false
	}
	items := make([]string, len(vs))
	for i, v := range vs {
		items[i] = g.arrayText(func() string { return g.element(t, v) })
	}
	return g.pool.array(elem, items), true
}

// rangeOf is storage built from every element of array name.
func rangeOf(storage, name string) string { return fmt.Sprintf(rangeFormat, storage, name, name) }

// hoistedLiteral is a list or map literal of a baked Build as its storage built from an array,
// false when it stays inline (DECISIONS 392).
func (g *gen) hoistedLiteral(t ir.TypeRef, v value.Value) (string, bool) {
	if text, ok := g.hoistedNested(t, v); ok {
		return text, true
	}
	switch x := v.(type) {
	case *value.List:
		if t.Kind != types.List || t.Elem == nil {
			return "", false
		}
		name, ok := g.hoistElems(*t.Elem, x.Elems)
		return rangeOf(g.storage(t), name), ok
	case *value.Map:
		return g.hoistedMap(t, x)
	}
	return "", false
}

// hoistedMap is a map literal as a FlatMap built from an array of its entries in insertion order.
func (g *gen) hoistedMap(t ir.TypeRef, m *value.Map) (string, bool) {
	if t.Kind != types.Map || t.Key == nil || t.Elem == nil || len(m.Keys) != len(m.Vals) || !g.hoists(len(m.Keys)) {
		return "", false
	}
	key, keyOK := g.arrayElem(*t.Key)
	val, valOK := g.arrayElem(*t.Elem)
	if !keyOK || !valOK {
		return "", false
	}
	items := make([]string, len(m.Keys))
	for i, k := range m.Keys {
		items[i] = g.arrayText(func() string {
			return openBrace + g.element(*t.Key, k) + listSep + g.element(*t.Elem, m.Vals[i]) + closeBrace
		})
	}
	name := g.pool.array(fmt.Sprintf(pairFormat, key, val), items)
	storage := g.storage(t)
	return fmt.Sprintf(fromArrayFormat, storage, rangeOf(fmt.Sprintf(entryVectorFormat, storage), name)), true
}

// hoistedKeys is a keyed list's n keys of type t, key(i) the i-th's literal, as a std::vector
// built from an array; "" when they stay inline.
func (g *gen) hoistedKeys(t ir.TypeRef, storage string, n int, key func(int) string) string {
	if !g.hoists(n) {
		return ""
	}
	elem, ok := g.arrayElem(t)
	if !ok {
		return ""
	}
	keys := make([]string, n)
	for i := range keys {
		keys[i] = g.arrayText(func() string { return key(i) })
	}
	return rangeOf(fmt.Sprintf(vectorFormat, storage), g.pool.array(elem, keys))
}

// hoistedEntries is a resolved list of refs as vec, its entries read by one loop from an array of their positions (CODEGEN.md §5.8).
func (g *gen) hoistedEntries(vec string, elem ir.TypeRef, l *value.List) (string, bool) {
	if !g.hoists(len(l.Elems)) {
		return "", false
	}
	at := make([]string, len(l.Elems))
	for i, e := range l.Elems {
		r, isRef := e.(*value.Ref)
		if !isRef {
			g.malformed(fmt.Sprintf(valueFormat, e), g.at)
			return "", false
		}
		pos, ok := g.bk.index[elem.Ref.Value][r.Key]
		if !ok {
			g.malformed(bakedEntryKey, g.at)
		}
		at[i] = strconv.Itoa(pos)
	}
	name := g.pool.array(cppSize, at)
	return fmt.Sprintf(entriesLoopFormat, vec, len(at), name, dataPrefix+g.pl.DataMember(elem.Ref.Value)), true
}

// hoistCells writes a lookup's cells, results of type t, into the array lhs from one array;
// false when they stay inline: too few, a type arrayElem refuses, a ref resolved into entries,
// or a cell the filler leaves as constructed (DECISIONS 392).
func (g *gen) hoistCells(a at, lhs string, t ir.TypeRef, optional bool, cells []value.Value) bool {
	if _, isRef := refSlot(t); isRef && g.pl.Resolves(t, nil) {
		return false
	}
	cell := t
	if optional {
		cell = ir.TypeRef{Kind: types.Optional, Elem: &t}
	}
	for _, c := range cells {
		if c == nil || !optional && absent(c) {
			return false
		}
	}
	name, ok := g.hoistElems(cell, cells)
	if ok {
		a.line(cellsCopyFormat, name, name, lhs)
	}
	return ok
}

// buildParts splits Build's statements into parts of about bakedPartLines lines, static members
// Build calls in order: their declarations, Build's body and their definitions; a body of one
// part stays in Build (DECISIONS 392).
func buildParts(access, body string) (decls, calls, defs string) {
	parts := splitBody(body)
	if len(parts) <= 1 {
		return "", body, ""
	}
	var d, c, f writer
	for i, p := range parts {
		name := ir.CppBuildPart(i)
		d.linef(1, partDeclFormat, name)
		c.linef(1, partCallFormat, name)
		f.printf(partDefFormat, access, name, p)
	}
	return d.String(), c.String(), f.String()
}

// splitBody cuts body into runs of at least bakedPartLines lines, each ending after a statement
// or block of Build's own level: a line indented once that opens nothing.
func splitBody(body string) []string {
	var parts []string
	var cur strings.Builder
	lines := 0
	for l := range strings.SplitSeq(body, newline) {
		if l == "" {
			continue
		}
		cur.WriteString(l)
		cur.WriteString(newline)
		lines++
		top := strings.HasPrefix(l, indentUnit) && !strings.HasPrefix(l, indentUnit+indentUnit)
		if lines >= bakedPartLines && top && !strings.HasSuffix(l, openBrace) {
			parts = append(parts, cur.String())
			cur.Reset()
			lines = 0
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// strForm is how strLit writes a string (DECISIONS 392).
type strForm uint8
