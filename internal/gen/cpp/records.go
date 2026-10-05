package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// classDecls writes the records, cases, variants and dependent types in dependency order (CODEGEN.md §5.4–§5.6).
func (g *gen) classDecls() {
	for _, c := range g.classes {
		leave := g.enter(c.canonName())
		switch {
		case c.dependent != nil:
			g.dependentClass(c.dependent)
		case c.row != nil:
			g.rowClassDecl(c.row)
		case c.variant != nil && c.cs == nil:
			g.variantClass(c.variant)
		default:
			g.recordClass(c)
		}
		leave()
	}
}

// recordClass is a record or case class: getters, methods, private members (CODEGEN.md §5.4, §7.2).
func (g *gen) recordClass(c class) {
	name := g.className(c)
	fields, fns := c.shape()
	if c.rec != nil {
		g.doc(0, c.rec.Doc)
		g.legacy(c.rec)
	} else {
		g.doc(0, c.cs.Doc)
	}
	sc := newScope(name)
	var members []string
	g.h.printf(classOpenFormat, name)
	if c.rec != nil && g.types() {
		g.publicDecodeDecl(sc, name)
	}
	if c.rec != nil && g.loaders[c.rec] != nil {
		g.fail(sc.add(ir.CppLoad, g.loaders[c.rec].Name))
		g.h.linef(1, loadDeclFormat, name)
	}
	if c.rec != nil && g.entries[c.rec] {
		idLine, idDecl := getIDLine, idMemberDecl
		if g.baked() && !g.pl.NestedRow(c.rec) {
			id := g.pl.IDName(c.rec)
			idLine, idDecl = fmt.Sprintf(getterFormat, id, idGetter, "", fmt.Sprintf(returnFormat, idMember)), fmt.Sprintf(memberFormat, id, idMember, initBraces)
		}
		g.getter(sc, idGetter, idLine, idMember, idDecl)
		g.getter(sc, retiredGetter, getRetiredLine, retiredMember, retiredMemberDecl)
		members = append(members, idDecl, retiredMemberDecl)
	}
	for _, f := range fields {
		members = append(members, g.fieldGetter(sc, c, f)...)
	}
	for _, fn := range fns {
		members = append(members, g.fnMember(sc, c, fields, fn)...)
	}
	var friends []string
	if c.rec != nil {
		friends = g.pairsFriends[c.rec]
	}
	g.private(name, len(members) > 0, friends...)
	for _, m := range members {
		g.h.lineAt(1, m)
	}
	g.h.line(closeClass)
	g.h.blank()
}

// legacy refuses a record mapped onto a hand-written struct: CODEGEN.md §7.8 is M6's, and stage E refuses it first (E8019 LegacyStruct, DECISIONS 320).
func (g *gen) legacy(r *ir.Record) {
	if r.Cpp.Struct != "" || r.Cpp.Access != ir.AccessNone {
		g.malformed(legacyStructs, r.Name)
	}
}

// private closes the public part and befriends the access struct and the decoder (§5.4, §7.2).
func (g *gen) private(name string, members bool, pairsParents ...string) {
	if !strings.HasSuffix(g.h.String(), publicLabel+newline) {
		g.h.blank()
	}
	g.h.line(privateLabel)
	g.h.linef(1, friendAccessFormat, g.pl.AccessName())
	g.h.linef(1, friendAccessFormat, g.pl.MakeStruct(g.p.Name))
	for _, n := range append([]string{name}, pairsParents...) {
		if !g.baked() {
			g.h.linef(1, friendDecodeFormat, n)
		}
	}
	if members {
		g.h.blank()
	}
}

// getter declares a getter and its member in the class scope, then writes the getter line.
func (g *gen) getter(sc *scope, name, line, mem, origin string) {
	g.fail(sc.add(name, origin))
	g.fail(sc.add(mem, origin))
	g.h.lineAt(1, line)
}

// fieldGetter writes a field's getters, the resolved one first (CODEGEN.md §2.6, §3.3, §5.8).
func (g *gen) fieldGetter(sc *scope, c class, f *ir.Field) []string {
	leave := g.enter(g.at + qnameSep + f.Name)
	defer leave()
	switch {
	case f.Input != nil && c.rec != nil:
		g.inputGetter(sc, c.rec, f)
		return nil
	case f.Input != nil:
		return nil
	case f.Type.Kind == types.Never && f.Optional:
		return nil
	}
	m, err := g.member(f.Name)
	g.fail(err)
	var refs []string
	doc := f.Doc
	if list, isRef := refSlot(f.Type); isRef {
		s := slot{member: m, ref: g.pl.RefMember(f.Name), wire: wireName(f), t: f.Type, optional: f.Optional, list: list}
		if target := g.resolvedTarget(s.target(), c); target != nil {
			g.doc(1, doc)
			doc = ""
			refs = append(refs, g.resolvedGetter(sc, g.baseName(f), access{cell: plain}, s, target))
			g.noteSlot(c, s, target)
		}
	}
	name := g.getterName(f)
	typ := g.getterType(f.Type, f.Optional)
	body, storage := fmt.Sprintf(returnFormat, m), g.memberType(f.Type, f.Optional)
	switch {
	case g.boxed[f]:
		body, storage = fmt.Sprintf(returnGetFormat, m), fmt.Sprintf(uniquePtrFormat, g.storage(f.Type))
	case f.Optional && !g.byValue(f.Type):
		body = fmt.Sprintf(returnPtrFormat, m)
	}
	g.doc(1, doc)
	g.getter(sc, name, fmt.Sprintf(getterFormat, typ, name, "", body), m, f.Name)
	members := append([]string{fmt.Sprintf(memberFormat, storage, m, g.memberInit(f.Type, f.Optional))}, refs...)
	return append(members, g.defineGetter(sc, f)...)
}

// plain is a cell that is the member itself.
func plain(m string) string { return m }

// baseName is Get + UpperCamel(f), or the field's @cpp(name:) (CODEGEN.md §3.3, §3.5).
func (g *gen) baseName(f *ir.Field) string {
	_, resolved := g.pl.FieldGetter(f)
	return resolved
}

// getterName is Get + UpperCamel(f) or @cpp(name:), then Key or Keys for refs (CODEGEN.md §3.3).
func (g *gen) getterName(f *ir.Field) string {
	getter, _ := g.pl.FieldGetter(f)
	return getter
}

// variantClass holds its cases in a std::variant; the kind is the index (CODEGEN.md §5.5).
func (g *gen) variantClass(v *ir.Variant) {
	name := g.typeName(v)
	kind := g.kindName(v)
	g.doc(0, v.Doc)
	g.h.printf(classOpenFormat, name)
	sc := newScope(name)
	if g.types() {
		g.publicDecodeDecl(sc, name)
	}
	g.h.linef(1, kindGetterFormat, kind, kind)
	g.fail(sc.add(kindGetter, v.Name))
	var alts []string
	for i, c := range v.Cases {
		if len(c.Fields) == 0 {
			alts = append(alts, cppMonostate)
			continue
		}
		cs := g.caseName(v, c)
		alts = append(alts, cs)
		as := g.pl.AsName(c)
		g.fail(sc.add(as, v.Name+qnameSep+c.Name))
		g.h.linef(1, asGetterFormat, cs, as, i)
	}
	g.private(name, true)
	g.h.linef(1, variantMemberFormat, strings.Join(alts, listSep))
	g.h.line(closeClass)
	g.h.blank()
}
