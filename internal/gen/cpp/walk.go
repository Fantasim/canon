package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// walks are the classes holding a resolved ref: each gets a Resolve overload (§5.9, §5.11).
func (g *gen) walks() []class {
	var out []class
	for _, c := range g.classes {
		if g.needsWalk(c.key()) {
			out = append(out, c)
		}
	}
	return out
}

// needsWalk reports a class holding, itself or through the classes it holds, a resolved slot.
func (g *gen) needsWalk(key any) bool { return g.reachesSlot(key, map[any]bool{}) }

func (g *gen) reachesSlot(key any, seen map[any]bool) bool {
	c, ok := g.classNamed(key)
	if !ok || seen[key] {
		return false
	}
	seen[key] = true
	if len(g.slots[key]) > 0 {
		return true
	}
	for _, to := range heldClasses(c) {
		if g.reachesSlot(to, seen) {
			return true
		}
	}
	return false
}

// heldClasses are the classes a class's fields (or a variant's cases) hold by value.
func heldClasses(c class) []any {
	var out []any
	if c.variant != nil && c.cs == nil {
		for _, cs := range c.variant.Cases {
			if len(cs.Fields) > 0 {
				out = append(out, cs)
			}
		}
		return out
	}
	fields, _ := c.shape()
	var deps []dep
	for _, f := range fields {
		deps = typeDeps(f.Type, true, deps)
	}
	for _, d := range deps {
		out = append(out, d.to)
	}
	return out
}

func (g *gen) classNamed(key any) (class, bool) {
	for _, c := range g.classes {
		if c.key() == key {
			return c, true
		}
	}
	return class{}, false
}

// ctxOf is where Resolve finds targets: the snapshot, or the holder's own container (§5.8).
func (g *gen) ctxOf(c class) (typ string, snapshot bool) {
	holders := g.holders[c.key()]
	if len(holders) == 0 || holders[0].Reload {
		return g.upper + snapshotSuffix, true
	}
	return containerName(holders[0]), false
}

// resolvers writes one Resolve overload per class of walks; a key naming no entry fails the
// load with the row, the field and the key (log-2026-09-24, gen/cpp round 3).
func (g *gen) resolvers(walks []class) {
	for _, c := range walks {
		ctx, snapshot := g.ctxOf(c)
		g.c.blank()
		g.c.linef(1, resolveOpenFormat, g.className(c), ctx)
		if c.variant != nil && c.cs == nil {
			g.resolveCases(c.variant)
		} else {
			g.resolveBody(c, snapshot)
		}
		g.c.linef(depthTwo, returnTrue)
		g.c.linef(1, closeBrace)
	}
}

// resolveCases resolves the case a variant holds, when that case holds a resolved ref.
func (g *gen) resolveCases(v *ir.Variant) {
	for i, cs := range v.Cases {
		if len(cs.Fields) > 0 && g.needsWalk(cs) {
			g.c.linef(depthTwo, resolveCaseFormat, i)
		}
	}
}

// resolveBody sets each resolved slot, then walks each field holding a class to resolve.
func (g *gen) resolveBody(c class, snapshot bool) {
	for _, r := range g.slots[c.key()] {
		find := findPrefix
		if snapshot {
			m, err := member(r.target.Name)
			g.fail(err)
			find = ctxPrefix + m + rowsFind
		}
		g.resolveSlot(r, find)
	}
	fields, _ := c.shape()
	for _, f := range fields {
		m, err := member(f.Name)
		g.fail(err)
		if !g.holdsWalk(f.Type) {
			continue
		}
		key := quote(wireName(f))
		if f.Inline {
			key = ""
		}
		g.walkValue(depthTwo, f.Type, f.Optional, xPrefix+m, key)
	}
}

// wireName is the key path a load error names for a field: its wire path, else its name.
func wireName(f *ir.Field) string {
	if len(f.WirePath) == 0 {
		return f.Name
	}
	return strings.Join(f.WirePath, qnameSep)
}

// resolveSlot points a slot at its entries, a list's one by one, through a present optional.
func (g *gen) resolveSlot(r resolved, find string) {
	key, ref, depth := xPrefix+r.member, xPrefix+r.member+refSuffix, depthTwo
	if r.cells > 0 {
		g.c.linef(depth, countForFormat, cellLoopVar, cellLoopVar, r.cells, cellLoopVar)
		key, ref, depth = fmt.Sprintf(indexFormat, key, cellLoopVar), fmt.Sprintf(indexFormat, ref, cellLoopVar), depth+1
	}
	text := g.keyText(r.slot.target().Key)
	switch {
	case r.list && r.optional:
		g.c.linef(depth, ifOpenFormat, key)
		g.c.linef(depth+1, emplaceFormat, refsVar, ref)
		g.resolveList(depth+1, fmt.Sprintf(derefFormat, key), refsVar, find, r)
		g.c.linef(depth, closeBrace)
	case r.list:
		g.resolveList(depth, key, ref, find, r)
	case r.optional:
		g.c.linef(depth, resolveOptionalFormat, key, ref, find, derefStar+key, quote(r.wire), fmt.Sprintf(text, derefStar+key))
	default:
		g.c.linef(depth, resolveFormat, ref, find, key, quote(r.wire), fmt.Sprintf(text, key))
	}
	if r.cells > 0 {
		g.c.linef(depthTwo, closeBrace)
	}
}

// resolveList resolves each key of keys into refs, a missing one named with its index.
func (g *gen) resolveList(depth int, keys, refs, find string, r resolved) {
	wire, text := r.wire, g.keyText(r.slot.target().Key)
	k := fmt.Sprintf(indexFormat, keys, indexLocal)
	g.c.linef(depth, forFormat, indexLocal, indexLocal, keys, indexLocal)
	g.c.linef(depth+1, findEntryFormat, find, k)
	g.c.linef(depth+1, failEntryFormat, elemKey(quote(wire), indexLocal), fmt.Sprintf(text, k))
	g.c.linef(depth+1, pushEntryFormat, refs)
	g.c.linef(depth, closeBrace)
}

// keyText is the format turning a key of type t into the text of a load error.
func (g *gen) keyText(t *ir.TypeRef) string {
	switch {
	case t == nil || t.Kind == types.String:
		return stringOfFormat
	case t.Kind == types.Enum:
		return fmt.Sprintf(stringOfFormat, fmt.Sprintf(helperFormat, toWireFunc, keyPlaceholder))
	default:
		return toStringFormat
	}
}

// holdsWalk reports a type holding, by value, a class to resolve.
func (g *gen) holdsWalk(t ir.TypeRef) bool {
	switch {
	case t.Kind == types.Record, t.Kind == types.Variant:
		return g.needsWalk(t.Named)
	case t.Kind == types.List && t.Elem != nil:
		return g.holdsWalk(*t.Elem)
	default:
		return false
	}
}

// walkValue resolves the classes a value of type t holds (directly, through an optional, or
// through the elements of a list or keyed list), with its key on the decoder's path.
func (g *gen) walkValue(depth int, t ir.TypeRef, optional bool, expr, key string) {
	switch {
	case optional:
		g.c.linef(depth, ifOpenFormat, expr)
		g.walkValue(depth+1, t, false, fmt.Sprintf(derefFormat, expr), key)
		g.c.linef(depth, closeBrace)
	case t.Kind == types.List:
		n := fmt.Sprintf(indexVarFormat, depth)
		elem, loop := fmt.Sprintf(indexFormat, expr, n), forFormat
		if t.KeyedBy != nil {
			elem, loop = fmt.Sprintf(keyedElemFormat, g.storage(*t.Elem), expr, n), lenLoopFormat
		}
		g.c.linef(depth, loop, n, n, expr, n)
		g.walkValue(depth+1, *t.Elem, false, elem, elemKey(key, n))
		g.c.linef(depth, closeBrace)
	case key == "":
		g.c.linef(depth, walkFormat, expr)
	default:
		g.c.linef(depth, pushFormat, key)
		g.c.linef(depth, walkFormat, expr)
		g.c.linef(depth, popLine)
	}
}

// resolveRows resolves a loaded value holding refs, rows[i] or value on the decoder's path.
func (g *gen) resolveRows(depth int, v *ir.Value, holder, ctx, fail string) {
	if v.Type.Kind == types.Record {
		g.c.linef(depth, pushFormat, quote(valueKey))
		g.c.linef(depth, resolveValueFormat, holder, ctx, fail)
		g.c.linef(depth, popLine)
		return
	}
	g.c.linef(depth, lenLoopFormat, indexLocal, indexLocal, holder+rowsMember, indexLocal)
	g.c.linef(depth+1, pushFormat, elemKey(quote(rowsKey), indexLocal))
	g.c.linef(depth+1, resolveRowFormat, g.storage(*v.Type.Elem), holder, indexLocal, ctx, fail)
	g.c.linef(depth+1, popLine)
	g.c.linef(depth, closeBrace)
}

// rootWalks reports a value whose root class holds a ref to resolve.
func (g *gen) rootWalks(v *ir.Value, walks []class) bool {
	root := v.Type
	if root.Kind != types.Record {
		root = *root.Elem
	}
	for _, c := range walks {
		if c.key() == any(root.Named) {
			return true
		}
	}
	return false
}
