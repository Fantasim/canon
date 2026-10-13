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

// openEnum writes an opened enum as a class holding its wire, one static const per member (retired ones too), ==, != and < by wire bytes, then k<E>Members, ToWire, ToName, <E>FromWire, IsKnown and, with @codes, ToCode and <E>FromCode; std::hash follows the namespace (CODEGEN.md §5.2, DECISIONS 362).
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
	g.h.blank()
	g.h.printf(openClassTailFormat, s.name, wire, ir.CppToWire)
	for _, m := range s.members {
		g.h.linef(0, openDefineFormat, s.name, m.name, quote(m.wire))
	}
	g.h.blank()
	qualified := make([]string, len(s.members))
	for i, m := range s.members {
		qualified[i] = s.name + scopeSep + m.name
	}
	g.h.printf(listConstFormat, fmt.Sprintf(arrayFormat, s.name, len(s.members)), g.pl.EnumHelpers(s.name).Members, braced(qualified))
	g.h.blank()
	g.h.printf(openToWireFormat, s.name, wire)
	g.openToName(s)
	g.fromWire(s)
	g.h.printf(openKnownFormat, s.name, g.pl.EnumHelpers(s.name).FromWire)
	if s.codes != nil {
		g.openToCode(s)
		g.fromCode(s)
	}
	g.afterNS.printf(openHashFormat, g.emit.Namespace+scopeSep+s.name, g.emit.Namespace+scopeSep)
}

// openToName is ToName: the Canon name of a known member, else the wire.
func (g *gen) openToName(s enumSpec) {
	g.h.printf(openNameOpenFormat, s.name)
	for _, m := range s.members {
		g.h.linef(1, openNameTestFormat, s.name+scopeSep+m.name, quote(m.canon))
	}
	g.h.linef(1, openNameCloseLine)
	g.h.line(closeBrace)
	g.h.blank()
}

// openToCode is ToCode: the code of a known member, else nothing.
func (g *gen) openToCode(s enumSpec) {
	code := intType(*s.codes)
	g.h.printf(openToCodeOpenFormat, code, s.name)
	for _, m := range s.members {
		g.h.linef(1, openToCodeTestFormat, s.name+scopeSep+m.name, code, m.code)
	}
	g.h.linef(1, returnNullopt)
	g.h.line(closeBrace)
	g.h.blank()
}
