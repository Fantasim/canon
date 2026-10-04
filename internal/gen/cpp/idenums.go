package cppgen

import (
	_ "embed"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

//go:embed text/id_from_wire.txt
var idFromWireText string

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

// idEnums write each table's id enum after the branch enums: its keys in entry order, retired ones included, an inline ToWire, and <Rec>IdFromWire, which the source defines (CODEGEN.md §2.7, §5.3, §7.3).
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
		g.h.printf(enumOpenFormat, s.name, s.underlying)
		for _, m := range s.members {
			g.h.linef(1, enumMemberFormat, m.name)
		}
		g.h.line(closeClass)
		g.h.blank()
		g.nameSwitch(s, ir.CppToWire, func(m enumMember) string { return m.wire })
		g.h.printf(idFromWireDeclFormat, s.name, g.pl.EnumHelpers(s.name).FromWire)
		g.h.blank()
		leave()
	}
}

// idFromWires define each <Rec>IdFromWire: a binary search of the keys sorted by their bytes (CODEGEN.md §5.3).
func (g *gen) idFromWires() {
	for _, it := range g.idTables() {
		name := g.pl.IDName(it.rec)
		keys := slices.Clone(it.v.IDs)
		slices.Sort(keys)
		quoted := make([]string, len(keys))
		ids := make([]string, len(keys))
		for i, k := range keys {
			quoted[i] = quote(k)
			ids[i] = name + scopeSep + g.pl.IDMember(k)
		}
		g.c.printf(idFromWireText, name, g.pl.EnumHelpers(name).FromWire, len(keys), indentBlock(braced(quoted)), indentBlock(braced(ids)))
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
	for _, v := range g.values {
		for _, r := range g.valueReaders(v) {
			g.declare(r.name, v.Name)
		}
	}
}
