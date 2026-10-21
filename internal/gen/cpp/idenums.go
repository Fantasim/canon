package cppgen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// idTable is a public table value with its row record: it has an id enum (CODEGEN.md §5.3).
type idTable struct {
	v   *ir.Value
	rec *ir.Record
}

// idTables are the package's public table values, emitted or not, in declaration order (CODEGEN.md §5.3).
func (g *gen) idTables() []idTable {
	var out []idTable
	for _, v := range g.p.Values {
		if v.Type.Kind != types.Table || v.Type.Elem == nil {
			continue
		}
		if rec, ok := v.Type.Elem.Named.(*ir.Record); ok {
			out = append(out, idTable{v: v, rec: rec})
		}
	}
	return out
}

// idEnums write each table's id enum after the branch enums: its keys in entry order, retired ones included, and the declarations of ToWire and <Rec>IdFromWire, which the source defines (CODEGEN.md §2.7, §5.3, §7.1, §7.3; DECISIONS 383).
func (g *gen) idEnums() {
	if !g.baked() {
		return
	}
	for _, it := range g.idTables() {
		leave := g.enter(it.v.Name)
		s := enumSpec{name: g.pl.IDName(it.rec), underlying: underlying(nil, len(it.v.IDs))}
		for _, id := range it.v.IDs {
			s.members = append(s.members, enumMember{name: g.pl.IDMember(id), wire: id})
		}
		g.doc(0, fmt.Sprintf(idEnumDocFormat, it.v.Name))
		g.h.printf(enumOpenFormat, s.name, s.underlying)
		for _, m := range s.members {
			g.h.linef(1, enumMemberFormat, m.name)
		}
		g.h.line(closeClass)
		g.h.blank()
		fromWire := g.pl.EnumHelpers(s.name).FromWire
		g.h.printf(declFormat, fmt.Sprintf(nameSigFormat, ir.CppToWire, s.name))
		g.h.printf(declFormat, fmt.Sprintf(idFromWireSigFormat, s.name, fromWire))
		g.h.blank()
		g.doc(0, fmt.Sprintf(idJSONDocFormat, fromWire))
		g.h.linef(0, idToJSONDeleted, s.name)
		g.h.linef(0, idFromJSONDeleted, s.name)
		g.h.blank()
		leave()
	}
}

// idFromWires define each id enum's ToWire, an array of its keys by position, then <Rec>IdFromWire, a binary search of the keys sorted by their bytes (CODEGEN.md §5.3, §7.1; DECISIONS 383).
func (g *gen) idFromWires() {
	for _, it := range g.idTables() {
		name := g.pl.IDName(it.rec)
		wires := views(it.v.IDs)
		pos := fmt.Sprintf(staticCastFormat, cppSize, enumArg)
		g.c.printf(byOrdinalText, fmt.Sprintf(nameSigFormat, ir.CppToWire, name), len(wires), wiresTable, indentBlock(braced(wires)), pos)
		keys := slices.Clone(it.v.IDs)
		slices.Sort(keys)
		quoted := views(keys)
		ids := make([]string, len(keys))
		for i, k := range keys {
			ids[i] = name + scopeSep + g.pl.IDMember(k)
		}
		g.c.write(search{
			sig: fmt.Sprintf(idFromWireSigFormat, name, g.pl.EnumHelpers(name).FromWire), prelude: table(cppStringView, keysTable, quoted) + table(name, idsTable, ids),
			keys: keysTable, key: keyParam, hit: fmt.Sprintf(indexFormat, idsTable, loVar), miss: cppNullopt,
		}.text())
	}
}

// indentBlock indents every line of a multi-line initializer after its first by one level.
func indentBlock(s string) string {
	return strings.ReplaceAll(s, newline, newline+indentUnit)
}

// declareBakedNames declares a baked emit's own namespace names: each id enum and its FromWire, each value's accessors (CODEGEN.md §3.5, §5.3, §5.9).
func (g *gen) declareBakedNames() {
	if !g.baked() {
		return
	}
	for _, it := range g.idTables() {
		name := g.pl.IDName(it.rec)
		g.declare(name, it.v.Name)
		g.declare(g.pl.EnumHelpers(name).FromWire, it.v.Name)
	}
	g.declareJSONHooks()
	for _, v := range g.values {
		for _, r := range g.valueReaders(v) {
			g.declare(r.name, v.Name)
		}
	}
}

// declareJSONHooks reserves the names of the deleted nlohmann hooks while a baked table has an id enum (CODEGEN.md §5.3; DECISIONS 379).
func (g *gen) declareJSONHooks() {
	tables := g.idTables()
	if len(tables) == 0 {
		return
	}
	g.declare(toJSONName, tables[0].v.Name)
	g.declare(fromJSONName, tables[0].v.Name)
}
