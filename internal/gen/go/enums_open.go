package gogen

import (
	"github.com/fantasim/canonlang/internal/ir"
)

// writeOpen writes an opened enum's methods after its `string` type and wire-valued constants: String (the Canon name of a known member, else the wire), Wire, Known, Parse<E> (E(wire) and false for an unknown one), <E>Members, and with @codes Code and <E>FromCode, each false for an unknown member (DECISIONS 339). The members sorted by wire are the one table of the enum; its names (only when one differs from its wire), its codes and the indexes of <E>Members and <E>FromCode refer to it (DECISIONS 393, 430).
func (g *gen) writeOpen(s *enumSpec) {
	v := g.enumView(s)
	v.String, v.Wire, v.Known, v.Code = ir.GoString, ir.GoWire, ir.GoKnown, ir.GoCode
	idx := byText(s.wires)
	at := positionsOf(idx)
	g.writeTable(v.Keys, s.name, pick(s.constNames(), idx))
	if v.Named {
		g.writeTable(v.Names, goString, pick(quoted(s.names), idx))
	}
	if s.codes != "" {
		g.writeOpenCodes(s, v, idx, at)
	}
	g.exec(tmplOpenEnum, v)
	g.writeRetired(s, v, v.Keys, idx)
	g.writeMembers(s, v, v.ByWireElem, indexLiterals(at))
	if s.codes != "" {
		g.exec(tmplOpenCodes, v)
	}
}

// writeOpenCodes writes the codes of an opened enum in the order of its keys and, when they are not dense in declaration order, the sorted codes with the positions in the keys of the members they belong to.
func (g *gen) writeOpenCodes(s *enumSpec, v enumView, idx, at []int) {
	g.writeTable(v.Numbers, s.codes, pick(s.numbers, idx))
	if !v.Sparse {
		return
	}
	byCode := s.byCode()
	g.writeTable(v.CodeKeys, s.codes, pick(s.numbers, byCode))
	g.writeTable(v.ByCode, v.ByWireElem, indexLiterals(pick(at, byCode)))
}
