package cppgen

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// pairsParents lists the decoders filling a record from `pairs:` slots: it befriends them.
func (g *gen) pairsParents() {
	for _, c := range g.declared() {
		fields, _ := c.shape()
		for _, f := range fields {
			rec := pairsElem(f)
			if rec != nil && rec.Pkg == g.p.Name {
				g.pairsFriends[rec] = append(g.pairsFriends[rec], g.className(c))
			} else if rec != nil {
				g.unsupported(foreignPairs, f.Name)
			}
		}
	}
}

// pairsElem is the two-field record of a `pairs:` field, or nil.
func pairsElem(f *ir.Field) *ir.Record {
	if f.Pairs == nil || f.Type.Kind != types.List || f.Type.Elem == nil {
		return nil
	}
	rec, _ := f.Type.Elem.Named.(*ir.Record)
	return rec
}

// decodePairs reads slots up to the first empty one: fields at k(i), v(i) (WIRE.md §5.14).
func (g *gen) decodePairs(f *ir.Field, dst string) {
	rec := pairsElem(f)
	if rec == nil || len(rec.Fields) != pairFields || f.Pairs.Slots <= 0 {
		g.fail(fmt.Errorf("%w: pairs field %s without a two-field record or slots", ErrMalformed, f.Name))
		return
	}
	g.c.linef(1, openBrace)
	for k, tmpl := range f.Pairs.Keys {
		keys := make([]string, f.Pairs.Slots)
		for i := range keys {
			keys[i] = quote(g.slotKey(tmpl, i))
		}
		g.c.linef(depthTwo, keysArrayFormat, fmt.Sprintf(wireKeysFormat, k), strings.Join(keys, listSep))
	}
	g.c.linef(depthTwo, emptyFlagLine)
	g.c.linef(depthTwo, slotLoopFormat, f.Pairs.Slots)
	g.c.linef(depthThree, slotCheckFormat, sourceVar, fmt.Sprintf(wireKeysFormat, 0), fmt.Sprintf(wireKeysFormat, 1))
	elem := g.storage(*f.Type.Elem)
	g.c.linef(depthThree, localFormat, elem, slotElem, "")
	for k, ef := range rec.Fields {
		m, err := member(ef.Name)
		g.fail(err)
		key := fmt.Sprintf(indexFormat, fmt.Sprintf(wireKeysFormat, k), slotVar)
		g.decodeKey(depthThree, sourceVar, key, leaf{t: ef.Type, unit: ef.Unit, enc: ef.Enc, dst: slotElem + memberAccess + m})
	}
	g.c.linef(depthThree, pushBackFormat, dst, slotElem)
	g.c.linef(depthTwo, closeBrace)
	g.c.linef(1, closeBrace)
}

// slotKey is a template with its `{i}` replaced by slot i in decimal (WIRE.md §5.14).
func (g *gen) slotKey(tmpl string, i int) string {
	before, after, found := strings.Cut(tmpl, openBrace)
	mark, rest, closed := strings.Cut(after, closeBrace)
	if !found || !closed || mark != slotLetter {
		g.fail(fmt.Errorf("%w: pairs template %s at %s", ErrMalformed, tmpl, g.at))
	}
	return before + strconv.Itoa(i) + rest
}

// decodeBits reads a `bits` list: the members whose code is set, in ascending code order (WIRE.md §5.3).
func (g *gen) decodeBits(depth int, src, key string, l leaf) {
	e, ok := l.t.Elem.Named.(*ir.Enum)
	if !ok || e.Codes == nil {
		g.fail(fmt.Errorf("%w: bits on a list of no @codes enum at %s", ErrMalformed, g.at))
		return
	}
	members := slices.Clone(e.Members)
	slices.SortStableFunc(members, func(a, b *ir.EnumMember) int { return cmp.Compare(a.Code, b.Code) })
	names := make([]string, len(members))
	for i, m := range members {
		names[i] = g.typeName(e) + scopeSep + enumerator(m)
	}
	enum := g.typeName(e)
	g.c.linef(depth, bitsTempLine)
	g.c.linef(depth, bitsOpenFormat, src, key, g.bitsMask(e))
	g.c.linef(depth+1, bitsArrayFormat, enum, strings.Join(names, listSep))
	g.c.linef(depth+1, bitsLoopFormat, enum)
	g.c.linef(depth+depthTwo, bitsTestFormat, l.dst)
	g.c.linef(depth+1, closeBrace)
	g.c.linef(depth, closeBrace)
}
