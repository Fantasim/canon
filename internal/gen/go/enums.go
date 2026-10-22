package gogen

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// enumSpec is a Canon enum, a variant's kind enum or a table's id enum (CODEGEN.md §5.2–§5.5).
type enumSpec struct {
	name     string
	doc      string
	under    string
	consts   []enumConst
	names    []string // String(): the Canon name, or the key of an id
	wires    []string // Wire(); nil for an id enum
	parseArg string   // parameter of Parse<E>: wire or key
	members  bool     // <E>Members()
	codes    string   // the @codes Go type: Code() and <E>FromCode
	open     bool     // an opened enum, whose values are its wire values (DECISIONS 339)
	numbers  []string // an opened enum's codes, in member order
	value    string   // an id enum: the table value its keys name (CODEGEN.md §5.3)
	id       bool     // an opaque id struct (a baked table's id), not an integer enum
	text     bool     // MarshalText and UnmarshalText of the wire value (§5.2, DECISIONS 379)
}

// enumConst is one member: its constant, its value and its doc.
type enumConst struct {
	name, value, doc string
	code             int64 // the @codes value, 0 without
	retired          bool
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
	defer g.enter(e.QName())()
	s := &enumSpec{name: g.goName(e), doc: e.Doc, parseArg: wireArg, members: true, open: ir.OpenEnum(g.p, g.e, e)}
	s.under = smallestUint(len(e.Members))
	if e.Codes != nil {
		s.codes = g.goType(*e.Codes)
		s.under = s.codes
	}
	if s.open {
		s.under = goString
	}
	s.text = !s.open && e.Codes == nil
	for i, m := range e.Members {
		v := strconv.Itoa(i)
		if e.Codes != nil {
			v = strconv.FormatInt(m.Code, decimal)
		}
		if s.open {
			s.numbers, v = append(s.numbers, v), strconv.Quote(m.Wire)
		}
		s.consts = append(s.consts, enumConst{g.names.MemberName(e, m), v, withRetired(m.Doc, m.Retired), m.Code, m.Retired})
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
		s := &enumSpec{name: g.names.KindName(v), under: smallestUint(len(v.Cases)), parseArg: wireArg, text: true}
		for i, c := range v.Cases {
			s.consts = append(s.consts, enumConst{name: g.names.KindMemberName(v, c), value: strconv.Itoa(i), doc: kindMemberDoc(c), retired: c.Retired})
			s.names = append(s.names, c.Name)
			s.wires = append(s.wires, c.Wire)
		}
		g.writeEnum(s)
	}
}

// kindMemberDoc is `Retired.` or nothing; a case without fields has no type for its doc (§2.6).
func kindMemberDoc(c *ir.Case) string {
	if len(c.Fields) > 0 {
		return withRetired("", c.Retired)
	}
	return withRetired(c.Doc, c.Retired)
}

// withRetired appends the line `Retired.` to the doc of a retired member (CODEGEN.md §5.2).
func withRetired(doc string, retired bool) string {
	if !retired {
		return doc
	}
	return strings.TrimPrefix(doc+newline+retiredDoc, newline)
}

// idEnums writes the id enum of every public table value (CODEGEN.md §5.3); in data and types mode a string type, once per name (DECISIONS 348).
func (g *gen) idEnums() {
	written := map[string]bool{}
	for _, v := range g.p.Values {
		if v.Type.Kind != types.Table {
			continue
		}
		rec, ok := g.sub(v.Type.Elem).Named.(*ir.Record)
		switch {
		case !ok:
			continue
		case g.isData():
			g.stringID(written, g.names.IDTypeName(rec)) // CODEGEN.md §5.3: data mode keys are strings
			continue
		}
		s := &enumSpec{name: g.names.IDTypeName(rec), under: smallestUint(len(v.IDs)), parseArg: keyArg, id: true, value: v.Name}
		for i, id := range v.IDs {
			s.consts = append(s.consts, enumConst{name: g.names.IDMemberName(rec, id), value: strconv.Itoa(i), retired: ir.EntryRetired(v, i)})
			s.names = append(s.names, id)
		}
		g.writeEnum(s)
	}
	g.nestedIDs(written)
}

// stringID writes the string id type name unless written holds it: tables whose id types are named alike share one (CODEGEN.md §5.3, DECISIONS 348).
func (g *gen) stringID(written map[string]bool, name string) {
	if !written[name] {
		written[name] = true
		g.printf(stringTypeFormat, name)
	}
}

// writeEnum writes the type, its constants and its methods.
func (g *gen) writeEnum(s *enumSpec) {
	if s.id {
		g.writeID(s)
		return
	}
	g.body.WriteString(docFor(s.name, s.doc))
	g.printf(enumHeaderFormat, s.name, s.under)
	for _, c := range s.consts {
		g.body.WriteString(docFor(c.name, c.doc))
		g.printf(enumConstFormat, c.name, s.name, c.value)
	}
	g.printf(closeParenFormat)
	if s.open {
		g.writeOpen(s)
		return
	}
	g.writeClosed(s)
}

// writeClosed writes the methods of an integer enum: String and Wire read tables by the member's index, or by its code when the values are @codes, and Parse<E> searches the sorted keys; the enum methods do not grow with the members (DECISIONS 393).
func (g *gen) writeClosed(s *enumSpec) {
	v := g.enumView(s)
	idx := s.positions()
	if s.codes != "" {
		g.writeTable(v.CodeKeys, s.name, pick(s.constNames(), idx))
	}
	g.writeText(v, v.Names, ir.GoString, pick(quoted(s.names), idx))
	if slices.Equal(s.names, s.wires) { // no wires table: the wires are the names
		g.textMethod(v, v.Names, ir.GoWire)
	} else {
		g.writeText(v, v.Wires, ir.GoWire, pick(quoted(s.wires), idx))
	}
	g.writeParse(s, v)
	g.writeClosedRetired(s, v, idx)
	if s.members {
		g.writeMembers(s, v)
	}
	if s.codes != "" {
		g.writeCodes(s, v)
	}
	if s.text {
		g.printf(enumTextFormat, s.name, v.Parse, g.use(strconvPkg, strconvPkg), g.use(errorsPkg, errorsPkg))
	}
}

// writeClosedRetired writes Retired: by index, or with @codes searched in the sorted codes (DECISIONS 395).
func (g *gen) writeClosedRetired(s *enumSpec, v enumView, idx []int) {
	search := ""
	if s.codes != "" {
		search = v.CodeKeys
	}
	g.writeRetired(s, v, search, idx)
}

// positions are the member indexes in the order the tables list them: declaration order, or by code when the values are the codes.
func (s *enumSpec) positions() []int {
	if s.codes != "" {
		return s.byCode()
	}
	return identity(len(s.consts))
}

// writeText writes a table of texts and the method (String or Wire) reading it.
func (g *gen) writeText(v enumView, table, method string, items []string) {
	g.writeTable(table, goString, items)
	g.textMethod(v, table, method)
}

// textMethod writes String or Wire over a table of texts, with the Go-style E(n) fallback.
func (g *gen) textMethod(v enumView, table, method string) {
	v.Method, v.Table, v.Strconv = method, table, g.use(strconvPkg, strconvPkg)
	g.exec(tmplEnumText, v)
}

// writeParse writes Parse<E>: a binary search of the wires (the keys of an id) sorted by their bytes, over the members in that order.
func (g *gen) writeParse(s *enumSpec, v enumView) {
	texts := s.wires
	if texts == nil {
		texts = s.names
	}
	idx := byText(texts)
	g.writeTable(v.Keys, goString, quoted(pick(texts, idx)))
	g.writeTable(v.Found, s.name, pick(s.memberRefs(), idx))
	g.exec(tmplEnumParse, v)
}

// writeMembers writes <E>Members over the members in declaration order, a table of the package.
func (g *gen) writeMembers(s *enumSpec, v enumView) {
	g.writeTable(v.Members, s.name, s.constNames())
	g.printf(membersFormat, g.names.MembersName(s.name), g.use(iterPkg, iterPkg), s.name, v.Members)
	g.writeLiveMembers(s, v)
}

// writeCodes writes Code and <E>FromCode of an enum whose values are its @codes (§5.2): the codes table is searched.
func (g *gen) writeCodes(s *enumSpec, v enumView) {
	g.printf(codesFormat, s.name, s.codes, ir.GoCode)
	g.exec(tmplEnumFromCode, v)
}
