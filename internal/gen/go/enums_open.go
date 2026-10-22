package gogen

import (
	"github.com/fantasim/canonlang/internal/ir"
)

// writeOpen writes an opened enum's methods after its `string` type and wire-valued constants: String (the Canon name of a known member, else the wire), Wire, Known, Parse<E> (E(wire) and false for an unknown one), <E>Members, and with @codes Code and <E>FromCode, each false for an unknown member (DECISIONS 339). Each searches a table of the members sorted by wire, or of the codes sorted (DECISIONS 393).
func (g *gen) writeOpen(s *enumSpec) {
	v := g.enumView(s)
	v.String, v.Wire, v.Known, v.Code = ir.GoString, ir.GoWire, ir.GoKnown, ir.GoCode
	idx := byText(s.wires)
	g.writeTable(v.Keys, s.name, pick(s.constNames(), idx))
	g.writeTable(v.Names, goString, pick(quoted(s.names), idx))
	if s.codes != "" {
		g.writeTable(v.Numbers, s.codes, pick(s.numbers, idx))
		byCode := s.byCode()
		g.writeTable(v.CodeKeys, s.codes, pick(s.numbers, byCode))
		g.writeTable(v.ByCode, s.name, pick(s.constNames(), byCode))
	}
	g.exec(tmplOpenEnum, v)
	g.writeRetired(s, v, v.Keys, idx)
	g.writeMembers(s, v)
	if s.codes != "" {
		g.exec(tmplOpenCodes, v)
	}
}
