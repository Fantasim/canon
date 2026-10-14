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

// open reports an enum the ts types-mode emit of its package opens, as this file sees it (CODEGEN.md §5.2, DECISIONS 362).
func (g *gen) open(e *ir.Enum) bool { return ir.OpenEnum(g.p, g.e, e) }

// closedName is the name of the closed union of an enum: `<E>Known` for an opened one, else the enum's own name, imported when another package's.
func (g *gen) closedName(e *ir.Enum) string {
	if !g.open(e) {
		return g.named(e)
	}
	known := ir.TSOpenEnumNames(e).Known
	if e.Pkg != g.p.Name {
		return g.importType(e.Pkg, known)
	}
	return known
}

func (g *gen) enum(e *ir.Enum) {
	defer g.enter(e.QName())()
	name := g.declare(typeName(e), e.QName())
	closed := name
	if g.open(e) {
		closed = g.declare(ir.TSOpenEnumNames(e).Known, e.QName())
	}
	members := make([]member, len(e.Members))
	for i, m := range e.Members {
		members[i] = member{wire: m.Wire, doc: m.Doc, retired: m.Retired}
	}
	var b strings.Builder
	if closed == name {
		b.WriteString(docComment("", e.Doc))
	}
	b.WriteString(g.union(closed, members))
	if closed != name {
		b.WriteString(docComment("", e.Doc) + fmt.Sprintf(openUnionFormat, name, closed))
	}
	b.WriteString(g.enumMembers(name, e))
	b.WriteString(g.enumRecord(name, e, namesSuffix, tsString, func(m *ir.EnumMember, _ int) string { return quote(m.Name) }))
	b.WriteString(g.enumRecord(name, e, indexSuffix, tsNumber, func(_ *ir.EnumMember, i int) string { return strconv.Itoa(i) }))
	if e.Codes != nil {
		b.WriteString(g.enumRecord(name, e, codesSuffix, tsNumber, func(m *ir.EnumMember, _ int) string {
			return strconv.FormatInt(m.Code, decimal)
		}))
	}
	if closed != name {
		b.WriteString(g.openFunctions(e, name))
	}
	g.add(b.String())
}

// openFunctions are `is<E>Known` and `<e>Name` of an opened enum (CODEGEN.md §5.2, DECISIONS 362).
func (g *gen) openFunctions(e *ir.Enum, name string) string {
	n := ir.TSOpenEnumNames(e)
	g.declare(n.IsKnown, e.QName())
	g.declare(n.Name, e.QName())
	return fmt.Sprintf(openFunctionsFormat, n.IsKnown, name, n.Known, typeName(e)+namesSuffix, n.Name)
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
	return fmt.Sprintf(membersFormat, name+membersSuffix, g.tableKey(name, e), strings.Join(items, listSep))
}

// enumRecord is `export const <E><suffix>: Readonly<Record<E, T>> = Object.freeze({ wire: value, ... });`.
func (g *gen) enumRecord(name string, e *ir.Enum, suffix, valueType string, item func(*ir.EnumMember, int) string) string {
	items := make([]string, len(e.Members))
	for i, m := range e.Members {
		items[i] = property(m.Wire) + keyValueSep + item(m, i)
	}
	g.declare(name+suffix, e.QName())
	return fmt.Sprintf(recordConstFormat, name+suffix, g.tableKey(name, e), valueType, strings.Join(items, listSep))
}

// tableKey is the type an enum's Members, Names, Index and Codes are keyed by: the enum, or `<E>Known` for an opened one.
func (g *gen) tableKey(name string, e *ir.Enum) string {
	if g.open(e) {
		return ir.TSOpenEnumNames(e).Known
	}
	return name
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

// idTypes writes the id type of every emitted table: the union of its keys and its Members in baked mode, string in data mode, once per name (CODEGEN.md §5.3, DECISIONS 348).
func (g *gen) idTypes() {
	written := map[string]bool{}
	for _, v := range g.emitted {
		elem := g.tableElem(v)
		switch {
		case elem == nil:
		case g.isData() && !written[idName(elem)]:
			written[idName(elem)] = true
			g.add(fmt.Sprintf(unionFormat, g.declare(idName(elem), v.Name), tsString))
		case !g.isData():
			g.idType(v, elem)
		}
	}
}

// idType writes a baked or embedded table's id type: the union of its keys and `<Id>Members` in entry order; no index is exported, a file that needs one builds it privately (CODEGEN.md §5.3, §8.2, DECISIONS 379).
func (g *gen) idType(v *ir.Value, elem ir.Type) {
	defer g.enter(v.Name)()
	name := g.declare(idName(elem), v.Name)
	members := make([]member, len(v.IDs))
	items := make([]string, len(v.IDs))
	for i, id := range v.IDs {
		members[i] = member{wire: id}
		items[i] = quote(id)
	}
	g.declare(name+membersSuffix, v.Name)
	g.declare(name+indexSuffix, v.Name)
	g.add(g.union(name, members) + fmt.Sprintf(membersFormat, name+membersSuffix, name, strings.Join(items, listSep)))
	g.ownIDItems[name] = len(g.items) - 1
}

// ownIDIndex is the private `<Id>Index` of this file's own table ids: the const joins the id's item the first time a lookup needs it (CODEGEN.md §5.3, §8.2).
func (g *gen) ownIDIndex(name string) string {
	index := name + indexSuffix
	if at, ok := g.ownIDItems[name]; ok && !g.ownIDDone[name] {
		g.ownIDDone[name] = true
		g.items[at] += newline + strings.TrimSuffix(fmt.Sprintf(privateIndexFormat, index, g.helper(canonIndexName), name+membersSuffix), newline)
	}
	return index
}

// foreignIDIndex is the private `<alias>_<Id>Index` of another package's table ids: `const ... = canonIndex(<alias>.<Id>Members)`, written once at the top of the declarations (CODEGEN.md §2.8, §5.3, DECISIONS 379).
func (g *gen) foreignIDIndex(r *ir.RefTarget) string {
	name := ir.TSForeignIDIndexName(r)
	if !g.foreignIDDone[name] {
		g.foreignIDDone[name] = true
		members := g.importValue(r.Pkg, idName(r.Elem)+membersSuffix)
		g.foreignIDs = append(g.foreignIDs, strings.TrimSuffix(fmt.Sprintf(privateIndexFormat, name, g.helper(canonIndexName), members), newline))
	}
	return name
}

// tableElem is the record a table value holds, or nil for a value that is not a table.
func (g *gen) tableElem(v *ir.Value) ir.Type {
	if v.Type.Kind != types.Table || v.Type.Elem == nil {
		return nil
	}
	return v.Type.Elem.Named
}
