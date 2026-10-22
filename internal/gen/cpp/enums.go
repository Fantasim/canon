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

// helperEnum is an enum whose helpers the .gen.cpp defines: a declared enum, opened or not, or a variant's kind enum, with the Canon item it comes from (CODEGEN.md §5.2, §5.5).
type helperEnum struct {
	spec   enumSpec
	open   bool
	origin string
}

// helperEnums are the declared enums in declaration order, then the kind enums of variants (CODEGEN.md §2.7).
func (g *gen) helperEnums() []helperEnum {
	var out []helperEnum
	for _, t := range g.p.Types {
		if e, ok := t.(*ir.Enum); ok {
			out = append(out, helperEnum{spec: g.declaredEnum(e), open: g.open(e), origin: e.Name})
		}
	}
	for _, t := range g.p.Types {
		if v, ok := t.(*ir.Variant); ok {
			out = append(out, helperEnum{spec: g.kindEnum(v), origin: v.Name})
		}
	}
	return out
}

// enums writes the declared enums, the kind enums of variants, the branch enums of dependent types, then a baked emit's id enums (CODEGEN.md §2.7, §5.2, §5.3, §5.5, §5.6).
func (g *gen) enums() {
	for _, he := range g.helperEnums() {
		leave := g.enter(he.origin)
		if he.open {
			g.openEnum(he.spec)
		} else {
			g.enum(he.spec)
		}
		leave()
	}
	g.branchEnums()
	g.idEnums()
}

func (g *gen) declaredEnum(e *ir.Enum) enumSpec {
	s := enumSpec{name: g.typeName(e), doc: e.Doc, codes: e.Codes}
	for _, m := range e.Members {
		s.members = append(s.members, enumMember{
			name: g.pl.Enumerator(m), canon: m.Name, wire: m.Wire, doc: m.Doc, code: m.Code, retired: m.Retired,
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
		s.members = append(s.members, enumMember{name: g.pl.KindMember(c), canon: c.Name, wire: c.Wire, doc: doc, retired: c.Retired})
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

// enum writes the declaration, k<E>Members, a @codes enum's ordinal, then the declarations of ToName, ToWire, <E>FromWire and <E>FromCode, which the .gen.cpp defines (CODEGEN.md §5.2, DECISIONS 383).
func (g *gen) enum(s enumSpec) {
	g.doc(0, s.doc)
	g.h.printf(enumOpenFormat, s.name, s.underlying)
	qualified, live := make([]string, len(s.members)), liveMembers(s)
	for i, m := range s.members {
		g.doc(1, m.doc)
		if m.retired {
			g.h.linef(1, docRetiredLine)
		}
		if s.codes != nil {
			g.h.linef(1, enumCodeFormat, m.name, intLit(m.code))
		} else {
			g.h.linef(1, enumMemberFormat, m.name)
		}
		qualified[i] = s.name + scopeSep + m.name
	}
	g.h.line(closeClass)
	g.h.blank()
	h := g.pl.EnumHelpers(s.name)
	g.h.printf(membersArrayFormat, s.name, len(s.members), h.Members, strings.Join(qualified, listSep))
	g.h.printf(membersArrayFormat, s.name, len(live), h.LiveMembers, strings.Join(live, listSep))
	g.h.blank()
	if s.codes != nil {
		g.ordinal(s)
	}
	for _, sig := range g.signatures(s, false).all() {
		g.h.printf(declFormat, sig)
	}
	g.h.blank()
}

// ordinal writes, in detail, a @codes enum's codes ascending, their declaration positions, and the constexpr search from a value to its position, the member count for a value that is no member (CODEGEN.md §5.2, DECISIONS 383).
func (g *gen) ordinal(s enumSpec) {
	h, code := g.pl.EnumHelpers(s.name), intType(*s.codes)
	order := byCode(s.members)
	codes, positions := make([]string, len(order)), make([]string, len(order))
	for i, at := range order {
		codes[i], positions[i] = intLit(s.members[at].code), fmt.Sprint(at)
	}
	g.h.line(detailOpen)
	g.h.printf(cellsFormat, code, len(order), h.Codes, braced(codes))
	g.h.printf(cellsFormat, cppUint32, len(order), h.CodeOrdinals, braced(positions))
	g.h.write(strings.TrimSuffix(search{
		sig: fmt.Sprintf(ordinalSigFormat, h.Ordinal, s.name), prelude: fmt.Sprintf(codeLineFormat, code),
		keys: h.Codes, key: codeField, hit: fmt.Sprintf(indexFormat, h.CodeOrdinals, loVar), miss: fmt.Sprintf(sizeCallFormat, h.Codes),
	}.text(), newline))
	g.h.line(detailClose)
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
	return g.kindName(v) + scopeSep + g.pl.KindMember(v.Cases[i])
}

// enumHelpers are e's k<E>Members, <E>FromWire, <E>FromCode and detail::<E>Ordinal, qualified when e is imported (CODEGEN.md §2.8, §5.2).
func (g *gen) enumHelpers(e *ir.Enum) ir.CppEnumHelpers {
	h, q := g.pl.EnumHelpers(g.pl.TypeName(e)), g.qualifier(e.Pkg)
	return ir.CppEnumHelpers{Members: q + h.Members, FromWire: q + h.FromWire, FromCode: q + h.FromCode, Ordinal: q + detailPrefix + h.Ordinal}
}

// liveMembers are the qualified members of s not retired, in members order (CODEGEN.md §5.2, DECISIONS 395).
func liveMembers(s enumSpec) []string {
	var out []string
	for _, m := range s.members {
		if !m.retired {
			out = append(out, s.name+scopeSep+m.name)
		}
	}
	return out
}
