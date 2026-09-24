package gogen

import (
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// enumSpec is a Canon enum, a variant's kind enum or a table's id enum (CODEGEN.md §5.2–§5.5).
type enumSpec struct {
	origin   string
	name     string
	doc      string
	under    string
	consts   []enumConst
	names    []string // String(): the Canon name, or the key of an id
	wires    []string // Wire(); nil for an id enum
	parseArg string   // parameter of Parse<E>: wire or key
	members  bool     // <E>Members()
	codes    string   // the @codes Go type: Code() and <E>FromCode
}

// enumConst is one member: its constant, its value and its doc.
type enumConst struct {
	name, value, doc string
}

// enums writes every enum in declaration order.
func (g *gen) enums() {
	for _, t := range g.p.Types {
		if e, ok := t.(*ir.Enum); ok {
			g.writeEnum(g.enumSpec(e))
		}
	}
}

func (g *gen) enumSpec(e *ir.Enum) *enumSpec {
	s := &enumSpec{origin: e.QName(), name: g.goName(e), doc: e.Doc, parseArg: wireArg, members: true}
	s.under = smallestUint(len(e.Members))
	if e.Codes != nil {
		s.codes = g.goType(*e.Codes)
		s.under = s.codes
	}
	for i, m := range e.Members {
		v := strconv.Itoa(i)
		if e.Codes != nil {
			v = strconv.FormatInt(m.Code, decimal)
		}
		doc := m.Doc
		if m.Retired {
			doc = strings.TrimPrefix(doc+newline+retiredDoc, newline)
		}
		s.consts = append(s.consts, enumConst{memberName(s.name, m), v, doc})
		s.names = append(s.names, m.Name)
		s.wires = append(s.wires, m.Wire)
	}
	return s
}

// smallestUint is the smallest of uint8, uint16, uint32 that holds n indexes (§5.2).
func smallestUint(n int) string {
	switch {
	case n <= maxUint8Indexes:
		return goUint8
	case n <= maxUint16Indexes:
		return goUint16
	}
	return goUint32
}

// kindEnums writes the kind enum of every variant (CODEGEN.md §5.5).
func (g *gen) kindEnums() {
	for _, t := range g.p.Types {
		v, ok := t.(*ir.Variant)
		if !ok {
			continue
		}
		kind := kindName(g.goName(v))
		s := &enumSpec{origin: v.QName(), name: kind, under: smallestUint(len(v.Cases)), parseArg: wireArg}
		for i, c := range v.Cases {
			s.consts = append(s.consts, enumConst{name: kind + upperCamel(c.Name), value: strconv.Itoa(i)})
			s.names = append(s.names, c.Name)
			s.wires = append(s.wires, c.Wire)
		}
		g.writeEnum(s)
	}
}

// idEnums writes the id enum of every public table value (CODEGEN.md §5.3).
func (g *gen) idEnums() {
	for _, v := range g.p.Values {
		if v.Type.Kind != types.Table {
			continue
		}
		rec, ok := g.sub(v.Type.Elem).Named.(*ir.Record)
		if !ok {
			continue
		}
		name := idTypeName(g.goName(rec))
		s := &enumSpec{origin: v.Name, name: name, under: smallestUint(len(v.IDs)), parseArg: keyArg}
		for i, id := range v.IDs {
			s.consts = append(s.consts, enumConst{name: idMemberName(name, id), value: strconv.Itoa(i)})
			s.names = append(s.names, id)
		}
		g.writeEnum(s)
	}
}

// writeEnum writes the type, its constants and its methods.
func (g *gen) writeEnum(s *enumSpec) {
	g.declare(s.name, s.origin)
	g.body.WriteString(docFor(s.name, s.doc))
	g.printf("type %s %s\n\nconst (\n", s.name, s.under)
	for i, c := range s.consts {
		g.declare(c.name, s.origin+dot+s.names[i])
		g.body.WriteString(docFor(c.name, c.doc))
		g.printf("%s %s = %s\n", c.name, s.name, c.value)
	}
	g.printf(")\n\n")
	g.writeSwitch(stringMethod, s, s.names)
	if s.wires != nil {
		g.writeSwitch(wireMethod, s, s.wires)
	}
	g.writeParse(s)
	if s.members {
		g.writeMembers(s)
	}
	if s.codes != "" {
		g.writeCodes(s)
	}
}

// writeSwitch writes String or Wire: a switch on the member, then the Go-style E(n) fallback.
func (g *gen) writeSwitch(method string, s *enumSpec, texts []string) {
	g.printf("func (self %s) %s() string {\nswitch self {\n", s.name, method)
	for i, c := range s.consts {
		g.printf("case %s:\nreturn %s\n", c.name, strconv.Quote(texts[i]))
	}
	g.printf(fallbackFormat, s.name, g.use(strconvPkg, strconvPkg))
}

func (g *gen) writeParse(s *enumSpec) {
	name := parsePrefix + s.name
	g.declare(name, s.origin)
	g.printf("func %s(%s string) (%s, bool) {\nswitch %s {\n", name, s.parseArg, s.name, s.parseArg)
	for i, c := range s.consts {
		text := s.names[i]
		if s.wires != nil {
			text = s.wires[i]
		}
		g.printf("case %s:\nreturn %s, true\n", strconv.Quote(text), c.name)
	}
	g.printf(notFoundTail)
}

func (g *gen) writeMembers(s *enumSpec) {
	name := s.name + membersSuffix
	g.declare(name, s.origin)
	names := make([]string, len(s.consts))
	for i, c := range s.consts {
		names[i] = c.name
	}
	g.printf(membersFormat, name, g.use(iterPkg, iterPkg), s.name, strings.Join(names, listSep))
}

// writeCodes writes Code and <E>FromCode of an enum whose values are its @codes (§5.2).
func (g *gen) writeCodes(s *enumSpec) {
	name := s.name + fromCodeSuffix
	g.declare(name, s.origin)
	g.printf(codesFormat, s.name, s.codes, name)
	for _, c := range s.consts {
		g.printf("case %s:\nreturn v, true\n", c.name)
	}
	g.printf(notFoundTail)
}
