package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// enumSpec is an enum as the header writes it: a declared enum or a variant's kind enum.
type enumSpec struct {
	name, doc  string
	underlying string
	members    []enumMember
	codes      *ir.TypeRef
}

// enumMember is an enumerator: its C++ name, its Canon name (ToName), its wire, its code.
type enumMember struct {
	name, canon, wire, doc string
	code                   int64
	retired                bool
}

// enums writes the declared enums, then the kind enums of variants (CODEGEN.md §2.7, §5.2, §5.5).
func (g *gen) enums() {
	for _, t := range g.p.Types {
		if e, ok := t.(*ir.Enum); ok {
			leave := g.enter(e.Name)
			g.enum(g.declaredEnum(e))
			leave()
		}
	}
	for _, t := range g.p.Types {
		if v, ok := t.(*ir.Variant); ok {
			leave := g.enter(v.Name)
			g.enum(g.kindEnum(v))
			leave()
		}
	}
}

func (g *gen) declaredEnum(e *ir.Enum) enumSpec {
	s := enumSpec{name: g.typeName(e), doc: e.Doc, codes: e.Codes}
	for _, m := range e.Members {
		s.members = append(s.members, enumMember{
			name: enumerator(m), canon: m.Name, wire: m.Wire, doc: m.Doc, code: m.Code, retired: m.Retired,
		})
	}
	s.underlying = underlying(e.Codes, len(s.members))
	return s
}

// kindEnum is a variant's kind enum; a fieldless case's doc goes on its member (decision 198).
func (g *gen) kindEnum(v *ir.Variant) enumSpec {
	s := enumSpec{name: g.kindName(v)}
	for _, c := range v.Cases {
		doc := ""
		if len(c.Fields) == 0 {
			doc = c.Doc
		}
		s.members = append(s.members, enumMember{name: kindMemberName(c), canon: c.Name, wire: c.Wire, doc: doc, retired: c.Retired})
	}
	s.underlying = underlying(nil, len(s.members))
	return s
}

// underlying is the @codes type, else the smallest of uint8_t, uint16_t, uint32_t that holds every index (§5.2).
func underlying(codes *ir.TypeRef, n int) string {
	switch {
	case codes != nil:
		return intType(*codes)
	case n <= maxUint8Members:
		return cppUint8
	case n <= maxUint16Members:
		return cppUint16
	}
	return cppUint32
}

// enum writes the declaration, k<E>Members, ToName, ToWire, <E>FromWire, <E>FromCode (CODEGEN.md §5.2).
func (g *gen) enum(s enumSpec) {
	g.doc(0, s.doc)
	g.h.printf(enumOpenFormat, s.name, s.underlying)
	qualified := make([]string, len(s.members))
	for i, m := range s.members {
		g.doc(1, m.doc)
		if m.retired {
			g.h.linef(1, docRetiredLine)
		}
		if s.codes != nil {
			g.h.linef(1, enumCodeFormat, m.name, m.code)
		} else {
			g.h.linef(1, enumMemberFormat, m.name)
		}
		qualified[i] = s.name + scopeSep + m.name
	}
	g.h.line(closeClass)
	g.h.blank()
	g.h.printf(membersArrayFormat, s.name, len(s.members), fmt.Sprintf(membersFormat, s.name), strings.Join(qualified, listSep))
	g.h.blank()
	g.nameSwitch(s, toNameFunc, func(m enumMember) string { return m.canon })
	g.nameSwitch(s, toWireFunc, func(m enumMember) string { return m.wire })
	g.fromWire(s)
	if s.codes != nil {
		g.fromCode(s)
	}
}

// nameSwitch is ToName or ToWire: a switch over the members, "" for a value that is none.
func (g *gen) nameSwitch(s enumSpec, fn string, text func(enumMember) string) {
	g.h.printf(nameFuncOpenFormat, fn, s.name)
	g.h.linef(1, switchFormat, enumArg)
	for _, m := range s.members {
		g.h.linef(1, caseReturnFormat, s.name+scopeSep+m.name, quote(text(m)))
	}
	g.h.linef(1, closeBrace)
	g.h.linef(1, returnEmpty)
	g.h.line(closeBrace)
	g.h.blank()
}

func (g *gen) fromWire(s enumSpec) {
	g.h.printf(fromWireOpenFormat, s.name, s.name+fromWireSuffix)
	for _, m := range s.members {
		g.h.linef(1, wireTestFormat, quote(m.wire), s.name+scopeSep+m.name)
	}
	g.h.linef(1, returnNullopt)
	g.h.line(closeBrace)
	g.h.blank()
}

func (g *gen) fromCode(s enumSpec) {
	g.h.printf(fromCodeOpenFormat, s.name, s.name+fromCodeSuffix, intType(*s.codes))
	g.h.linef(1, switchFormat, codeField)
	for _, m := range s.members {
		g.h.linef(1, caseReturnFormat, fmt.Sprint(m.code), s.name+scopeSep+m.name)
	}
	g.h.linef(1, closeBrace)
	g.h.linef(1, returnNullopt)
	g.h.line(closeBrace)
	g.h.blank()
}

// doc writes a doc comment at depth: `/// ` + line, `///` for an empty line (CODEGEN.md §2.6).
func (g *gen) doc(depth int, text string) {
	if text == "" {
		return
	}
	for l := range strings.SplitSeq(text, newline) {
		if l == "" {
			g.h.linef(depth, docEmpty)
			continue
		}
		g.h.linef(depth, docLineFormat, l)
	}
}

// kindMember is the qualified kind-enum member of case i (CODEGEN.md §3.3, §3.5).
func (g *gen) kindMember(v *ir.Variant, i int) string {
	return g.kindName(v) + scopeSep + kindMemberName(v.Cases[i])
}

// kindMemberName is a case's kind member: its @cpp(name:), else its name verbatim (§3.5).
func kindMemberName(c *ir.Case) string {
	return override(ir.NameOptions{Name: c.Cpp.Name}, verbatim(c.Name))
}
