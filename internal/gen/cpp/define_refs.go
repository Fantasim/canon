package cppgen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// defineTables writes each define table the fields ref, sorted by name, in the .gen.cpp's detail namespace before the decoders that search it (CODEGEN.md §5.8).
func (g *gen) defineTables() {
	for _, d := range ir.OwnDefines(g.p) {
		if len(d.Names) != len(d.Values) {
			g.malformed(defineMissing, d.Pkg+qnameSep+d.Value)
			continue
		}
		g.c.printf(definesOpenFormat, len(d.Names), g.pl.DefinesName(d))
		for i, n := range d.Names {
			g.c.linef(1, defineEntryFormat, quote(n), intLit(d.Values[i]))
		}
		g.c.line(definesClose)
		g.c.blank()
	}
}

// defineValueType is what a define ref's value members hold: an int64_t, or one per key of a list (CODEGEN.md §5.8).
func defineValueType(t ir.TypeRef) ir.TypeRef {
	value := ir.TypeRef{Kind: types.Int, Bits: bits64, Signed: true}
	if t.Kind == types.List {
		return ir.TypeRef{Kind: types.List, Elem: &value}
	}
	return value
}

// defineGetter declares a define ref's value getter, Get<F>Value(s), and returns its member (CODEGEN.md §3.3, §5.8).
func (g *gen) defineGetter(sc *scope, f *ir.Field) []string {
	name, member, ok := g.pl.DefineValue(f)
	if !ok {
		return nil
	}
	t := defineValueType(f.Type)
	body := fmt.Sprintf(returnFormat, member)
	if f.Optional && !byValue(t) {
		body = fmt.Sprintf(returnPtrFormat, member)
	}
	g.doc(1, f.Doc)
	g.getter(sc, name, fmt.Sprintf(getterFormat, g.getterType(t, f.Optional), name, "", body), member, f.Name)
	return []string{fmt.Sprintf(memberFormat, g.memberType(t, f.Optional), member, memberInit(t, f.Optional))}
}

// defineLookup looks a define ref's keys up once read, key naming them in load errors (CODEGEN.md §5.8).
func (g *gen) defineLookup(depth int, f *ir.Field, recv, key string) {
	_, member, ok := g.pl.DefineValue(f)
	if !ok {
		return
	}
	d := ir.DefinesOf(g.p, ir.DefineTarget(f.Type))
	m, err := g.member(f.Name)
	g.fail(err)
	if d == nil {
		g.malformed(defineMissing, g.at)
		return
	}
	keys, values := recv+m, recv+member
	call := func(depth int, key, name, dst string) {
		g.c.linef(depth, defineCallFormat, g.pl.DefinesName(d), quote(ir.DefineTableName(d)), key, name, dst)
	}
	i := fmt.Sprintf(indexVarFormat, depth)
	switch {
	case f.Type.Kind != types.List && !f.Optional:
		call(depth, key, keys, values)
	case f.Type.Kind != types.List:
		g.c.linef(depth, ifOpenFormat, keys)
		call(depth+1, key, derefStar+keys, values+emplaceCall)
		g.c.linef(depth, closeBrace)
	case !f.Optional:
		g.c.linef(depth, forFormat, i, i, keys, i)
		call(depth+1, elemKey(key, i), fmt.Sprintf(indexFormat, keys, i), values+emplaceBackCall)
		g.c.linef(depth, closeBrace)
	default:
		vs := fmt.Sprintf(optVarFormat, depth)
		g.c.linef(depth, ifOpenFormat, keys)
		g.c.linef(depth+1, emplaceFormat, vs, values)
		g.c.linef(depth+1, forFormat, i, i, fmt.Sprintf(derefFormat, keys), i)
		call(depth+depthTwo, elemKey(key, i), fmt.Sprintf(indexFormat, fmt.Sprintf(derefFormat, keys), i), vs+emplaceBackCall)
		g.c.linef(depth+1, closeBrace)
		g.c.linef(depth, closeBrace)
	}
}
