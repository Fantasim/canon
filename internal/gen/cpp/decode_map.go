package cppgen

import (
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// decodeMap reads an object as a canon::FlatMap, its members in file order, each key as the wire key WIRE.md §5.8 gives its type; the data loader reads the file's text, so order survives (CODEGEN.md §5.9, §5.13, DECISIONS 312).
func (g *gen) decodeMap(depth int, src, key string, l leaf) {
	if g.types() || l.t.Key == nil || l.t.Elem == nil {
		g.malformed(mapFields, g.at) // E8019 MapField
		return
	}
	kt, vt := *l.t.Key, *l.t.Elem
	ks, vs := g.storage(kt), g.storage(vt)
	d, inner, body := depth, depth+1, depth+depthTwo
	g.c.linef(depth, pushFormat, key)
	g.c.linef(depth, mapMembersFormat, d)
	g.c.linef(depth, mapReadFormat, src, d)
	g.c.linef(inner, mapEntriesFormat, ks, vs, d)
	g.c.linef(inner, mapLoopFormat, d)
	g.c.linef(body, localFormat, ks, fmt.Sprintf(mapKeyLocal, d), elemInit(kt))
	g.c.linef(body, localFormat, vs, fmt.Sprintf(mapValueLocal, d), elemInit(vt))
	g.mapKey(body, d, kt)
	if k := vt.Kind; k != types.Record && k != types.Variant {
		g.c.linef(body, nullElemFormat, fmt.Sprintf(mapValueExpr, d), fmt.Sprintf(mapKeyExpr, d))
	}
	g.decodeValue(body, fmt.Sprintf(mapValueExpr, d), fmt.Sprintf(mapKeyExpr, d), leaf{t: vt, unit: l.unit, enc: l.enc, dst: fmt.Sprintf(mapValueLocal, d), disc: l.disc})
	g.c.linef(body, mapEmplaceFormat, d)
	g.c.linef(inner, closeBrace)
	g.c.linef(inner, mapFromFormat, l.dst, ks, vs, d)
	g.c.linef(depth, closeBrace)
	g.c.linef(depth, popLine)
}

// mapKey reads the key of member d into its local: a canonical decimal integer or the text itself as a JSON value, decoded as a value of the key type.
func (g *gen) mapKey(depth, d int, kt ir.TypeRef) {
	if ir.NumericMapKey(kt) {
		g.c.linef(depth, mapIntKeyFormat, d)
		g.c.linef(depth, mapIntCheckFmt, d)
	} else {
		g.c.linef(depth, mapTextKeyFormat, d)
	}
	g.decodeValue(depth, fmt.Sprintf(mapJSONKeyFormat, d), fmt.Sprintf(mapKeyExpr, d), leaf{t: kt, dst: fmt.Sprintf(mapKeyLocal, d)})
}

// walkMap checks the ref keys of a map name an entry and resolves the classes its values hold, each member in turn, on the path its key gives (WIRE.md §5.8, §5.9).
func (g *gen) walkMap(depth int, t ir.TypeRef, expr, key string) {
	n := fmt.Sprintf(indexVarFormat, depth)
	kt := *t.Key
	for kt.Kind == types.Ref && kt.Key != nil {
		kt = *kt.Key
	}
	entry := fmt.Sprintf(mapEntryFormat, expr, n)
	text := fmt.Sprintf(g.keyText(&kt), entry+entryFirst)
	at := fmt.Sprintf(mapPathFormat, key, text)
	g.c.linef(depth, lenLoopFormat, n, n, expr, n)
	if target := g.keyTarget(t, g.walking); target != nil {
		g.c.linef(depth+1, mapKeyCheckFmt, g.findFor(target), entry+entryFirst, at, text)
	}
	if g.holdsWalk(*t.Elem) {
		g.walkValue(depth+1, *t.Elem, false, fmt.Sprintf(mapElemFormat, g.storage(*t.Elem), expr, n), at)
	}
	g.c.linef(depth, closeBrace)
}

// keyTarget is the value the ref keys of map t resolve into at load, in class c: nil when t's key is no ref or its target is none c's holders resolve into (CODEGEN.md §5.8), where the key is read unchecked.
func (g *gen) keyTarget(t ir.TypeRef, c class) *ir.Value {
	if t.Kind != types.Map || t.Key == nil || t.Key.Kind != types.Ref || t.Key.Ref == nil {
		return nil
	}
	return g.resolvedTarget(*t.Key, c)
}

// checksKeys reports a class one of whose fields or stored or lookup results holds a map whose ref keys the loader checks.
func (g *gen) checksKeys(c class) bool {
	fields, fns := c.shape()
	for _, f := range fields {
		if g.keyChecked(f.Type, c) {
			return true
		}
	}
	return slices.ContainsFunc(fns, func(fn *ir.ExportFn) bool { return fn.Kind != ir.FnTranslated && g.keyChecked(fn.Result, c) })
}

// keyChecked reports a map, t itself or one its list, optional or map elements are, whose ref keys the loader checks in class c.
func (g *gen) keyChecked(t ir.TypeRef, c class) bool {
	for ; ; t = *t.Elem {
		if g.keyTarget(t, c) != nil {
			return true
		}
		if t.Elem == nil || t.Kind != types.List && t.Kind != types.Optional && t.Kind != types.Map {
			return false
		}
	}
}
