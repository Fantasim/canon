package tsgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// enums writes every enum in declaration order: its union, and the Members, Names and Index tables, and Codes for `@codes` (CODEGEN.md §5.2).
func (g *gen) enums() {
	for _, t := range g.p.Types {
		if e, ok := t.(*ir.Enum); ok {
			g.enum(e)
		}
	}
}

func (g *gen) enum(e *ir.Enum) {
	defer g.enter(e.QName())()
	name := g.declare(typeName(e), e.QName())
	members := make([]member, len(e.Members))
	for i, m := range e.Members {
		members[i] = member{wire: m.Wire, doc: m.Doc, retired: m.Retired}
	}
	var b strings.Builder
	b.WriteString(docComment("", e.Doc))
	b.WriteString(g.union(name, members))
	b.WriteString(g.enumMembers(name, e))
	b.WriteString(g.enumRecord(name, e, namesSuffix, tsString, func(m *ir.EnumMember, _ int) string { return quote(m.Name) }))
	b.WriteString(g.enumRecord(name, e, indexSuffix, tsNumber, func(_ *ir.EnumMember, i int) string { return strconv.Itoa(i) }))
	if e.Codes != nil {
		b.WriteString(g.enumRecord(name, e, codesSuffix, tsNumber, func(m *ir.EnumMember, _ int) string {
			return strconv.FormatInt(m.Code, decimal)
		}))
	}
	g.add(b.String())
}

// member is one string of a union, with the doc its line carries (CODEGEN.md §2.6, §5.2).
type member struct {
	wire, doc string
	retired   bool
}

// union is `export type name = "a" | "b";` on one line, or one member per line when a member has a doc; `never` when it has none (an empty table's ids).
func (g *gen) union(name string, members []member) string {
	documented := false
	for _, m := range members {
		documented = documented || m.doc != "" || m.retired
	}
	if len(members) == 0 {
		return fmt.Sprintf(unionFormat, name, tsNever)
	}
	if !documented {
		parts := make([]string, len(members))
		for i, m := range members {
			parts[i] = quote(m.wire)
		}
		return fmt.Sprintf(unionFormat, name, strings.Join(parts, unionSep))
	}
	var b strings.Builder
	fmt.Fprintf(&b, unionOpenFormat, name)
	for _, m := range members {
		b.WriteString(docComment(indent, memberDoc(m)))
		fmt.Fprintf(&b, unionMemberFormat, quote(m.wire))
	}
	return strings.TrimSuffix(b.String(), newline) + semicolon + newline
}

// memberDoc is a member's doc, with `Retired.` added for a retired one (CODEGEN.md §5.2).
func memberDoc(m member) string {
	if !m.retired {
		return m.doc
	}
	if m.doc == "" {
		return fmt.Sprintf(retiredFormat, "")
	}
	return fmt.Sprintf(retiredFormat, m.doc+newline)
}

// enumMembers is `export const <E>Members: ReadonlyArray<E> = Object.freeze([...]);`.
func (g *gen) enumMembers(name string, e *ir.Enum) string {
	items := make([]string, len(e.Members))
	for i, m := range e.Members {
		items[i] = quote(m.Wire)
	}
	g.declare(name+membersSuffix, e.QName())
	return fmt.Sprintf(membersFormat, name+membersSuffix, name, strings.Join(items, listSep))
}

// enumRecord is `export const <E><suffix>: Readonly<Record<E, T>> = Object.freeze({ wire: value, ... });`.
func (g *gen) enumRecord(name string, e *ir.Enum, suffix, valueType string, item func(*ir.EnumMember, int) string) string {
	items := make([]string, len(e.Members))
	for i, m := range e.Members {
		items[i] = property(m.Wire) + keyValueSep + item(m, i)
	}
	g.declare(name+suffix, e.QName())
	return fmt.Sprintf(recordConstFormat, name+suffix, name, valueType, strings.Join(items, listSep))
}

// kindEnums writes the kind union of every variant: the wire names of its cases (CODEGEN.md §5.5).
func (g *gen) kindEnums() {
	for _, t := range g.p.Types {
		if v, ok := t.(*ir.Variant); ok {
			g.kindEnum(v)
		}
	}
}

func (g *gen) kindEnum(v *ir.Variant) {
	defer g.enter(v.QName())()
	members := make([]member, len(v.Cases))
	for i, c := range v.Cases {
		members[i] = member{wire: c.Wire, retired: c.Retired}
		if len(c.Fields) == 0 {
			members[i].doc = c.Doc
		}
	}
	g.add(g.union(g.declare(kindName(v), v.QName()), members))
}

// branchEnums writes the branch union of every dependent type: its branch names in arm order (CODEGEN.md §5.6).
func (g *gen) branchEnums() {
	for _, t := range g.p.Types {
		d, ok := t.(*ir.Dependent)
		if !ok {
			continue
		}
		members := make([]member, len(d.Branches))
		for i, b := range d.Branches {
			members[i] = member{wire: b.Name}
		}
		g.add(g.union(g.declare(branchName(d), d.QName()), members))
	}
}

// idTypes writes the id type of every emitted table: the union of its keys and its Index in baked mode, string in data mode (CODEGEN.md §5.3).
func (g *gen) idTypes() {
	for _, v := range g.emitted {
		if elem := g.tableElem(v); elem != nil {
			g.idType(v, elem)
		}
	}
}

func (g *gen) idType(v *ir.Value, elem ir.Type) {
	defer g.enter(v.Name)()
	name := g.declare(idName(elem), v.Name)
	if g.isData() {
		g.add(fmt.Sprintf(unionFormat, name, tsString))
		return
	}
	members := make([]member, len(v.IDs))
	items := make([]string, len(v.IDs))
	for i, id := range v.IDs {
		members[i] = member{wire: id}
		items[i] = property(id) + keyValueSep + strconv.Itoa(i)
	}
	g.declare(name+indexSuffix, v.Name)
	index := emptyObject
	if len(items) > 0 {
		index = lbrace + space + strings.Join(items, listSep) + space + rbrace
	}
	g.add(g.union(name, members) + fmt.Sprintf(indexConstFormat, name+indexSuffix, name, index))
}

// tableElem is the record a table value holds, or nil for a value that is not a table.
func (g *gen) tableElem(v *ir.Value) ir.Type {
	if v.Type.Kind != types.Table || v.Type.Elem == nil {
		return nil
	}
	return v.Type.Elem.Named
}
