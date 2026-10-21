package cppgen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// open reports an enum the cpp emit opens (CODEGEN.md §5.2, DECISIONS 362): a class holding its wire.
func (g *gen) open(e *ir.Enum) bool { return ir.OpenEnum(g.p, g.emit, e) }

// openPlain reports a field, element or key of an opened enum that a decoder reads from any JSON string: not one on a numeric wire, which stays closed (CODEGEN.md §5.2).
func (g *gen) openPlain(t ir.TypeRef) bool {
	e, ok := t.Named.(*ir.Enum)
	return ok && t.Kind == types.Enum && !e.JSONCodes && g.open(e)
}

// openEnum writes an opened enum as a class holding its wire, one static const per member (retired ones too), ==, != and < by wire bytes, then k<E>Members, the inline ToWire and the declarations of ToName, IsKnown, <E>FromWire and, with @codes, ToCode and <E>FromCode, which the .gen.cpp defines; std::hash follows the namespace (CODEGEN.md §5.2, DECISIONS 362, 383).
func (g *gen) openEnum(s enumSpec) {
	wire := ir.CppOpenWire
	g.doc(0, s.doc)
	g.h.printf(openClassOpenFormat, s.name, wire)
	for _, m := range s.members {
		g.doc(1, m.doc)
		if m.retired {
			g.h.linef(1, docRetiredLine)
		}
		g.h.linef(1, openMemberFormat, s.name, m.name)
	}
	if len(s.members) > 0 {
		g.h.blank()
	}
	g.h.printf(openClassTailFormat, s.name, wire, ir.CppToWire)
	for _, m := range s.members {
		g.h.linef(0, openDefineFormat, s.name, m.name, stringLit(m.wire))
	}
	if len(s.members) > 0 {
		g.h.blank()
	}
	qualified := make([]string, len(s.members))
	for i, m := range s.members {
		qualified[i] = s.name + scopeSep + m.name
	}
	g.h.printf(listConstFormat, fmt.Sprintf(arrayFormat, s.name, len(s.members)), g.pl.EnumHelpers(s.name).Members, braced(qualified))
	g.h.blank()
	g.h.printf(openToWireFormat, s.name, wire)
	for _, sig := range g.signatures(s, true).all() {
		g.h.printf(declFormat, sig)
	}
	g.h.blank()
	g.afterNS.printf(openHashFormat, g.emit.Namespace+scopeSep+s.name, g.emit.Namespace+scopeSep)
}
