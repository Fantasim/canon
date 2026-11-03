package cppgen

import (
	"fmt"

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

// idEnums write each table's id enum after the branch enums: its keys in entry order, retired ones included, the declarations of ToWire and <Rec>IdFromWire, which the source defines, and the deleted to_json and from_json pair (CODEGEN.md §2.7, §5.3, §7.1, §7.3; DECISIONS 383).
func (g *gen) idEnums() {
	if !g.baked() {
		return
	}
	for _, it := range g.idTables() {
		leave := g.enter(it.v.Name)
		s := g.idSpec(it)
		g.doc(0, fmt.Sprintf(idEnumDocFormat, it.v.Name))
		g.h.printf(enumOpenFormat, s.name, s.underlying)
		for _, m := range s.members {
			g.h.linef(1, enumMemberFormat, m.name)
		}
		g.h.line(closeClass)
		g.h.blank()
		fromWire := g.pl.EnumHelpers(s.name).FromWire
		g.h.printf(declFormat, fmt.Sprintf(nameSigFormat, ir.CppToWire, s.name))
		g.h.printf(declFormat, fmt.Sprintf(retiredSigFormat, s.name))
		g.h.printf(declFormat, fmt.Sprintf(idFromWireSigFormat, s.name, fromWire))
		g.deletedJSON(s.name, fmt.Sprintf(idJSONDocFormat, fromWire))
		g.h.blank()
		leave()
	}
}

// idSpec is the id enum of a table: its keys in entry order, retired ones included (CODEGEN.md §5.3).
func (g *gen) idSpec(it idTable) enumSpec {
	s := enumSpec{name: g.pl.IDName(it.rec), underlying: underlying(nil, len(it.v.IDs)), id: true}
	for i, id := range it.v.IDs {
		s.members = append(s.members, enumMember{name: g.pl.IDMember(id), wire: id, retired: ir.EntryRetired(it.v, i)})
	}
	return s
}

// idFromWires define each id enum's tables, its ToWire, an array of its keys by position, then <Rec>IdFromWire, a binary search of the positions sorted by key (CODEGEN.md §5.3, §7.1; DECISIONS 383, 430).
func (g *gen) idFromWires() {
	for _, it := range g.idTables() {
		s := g.idSpec(it)
		h := g.pl.EnumHelpers(s.name)
		g.enumTables(s)
		pos := fmt.Sprintf(staticCastFormat, cppSize, enumArg)
		wires, order := detailPrefix+h.Wires, detailPrefix+h.ByWire
		g.c.printf(byOrdinalText, fmt.Sprintf(nameSigFormat, ir.CppToWire, s.name), pos, wires)
		g.c.printf(retiredText, fmt.Sprintf(retiredSigFormat, s.name), detailPrefix+h.Retired, pos)
		hit := fmt.Sprintf(staticCastFormat, s.name, fmt.Sprintf(indexFormat, order, loVar))
		g.c.write(byWireSearch(fmt.Sprintf(idFromWireSigFormat, s.name, h.FromWire), keyParam, wires, order, hit))
	}
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
	for _, v := range g.values {
		for _, r := range g.valueReaders(v) {
			g.declare(r.name, v.Name)
		}
	}
}

// declareJSONHooks reserves the names of the deleted nlohmann hooks while a baked table has an id enum or any enum, kind or branch enum has positions for values (CODEGEN.md §3.5, §5.2, §5.3; DECISIONS 379, 402).
func (g *gen) declareJSONHooks() {
	origin := g.pl.JSONHooksOrigin()
	if origin == "" {
		return
	}
	g.declare(toJSONName, origin)
	g.declare(fromJSONName, origin)
}
