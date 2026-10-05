package cppgen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// rowClass is the class of a table's rows of rec, as the package whose class holds the table lays it out: rec itself when rec is that package's, else that package's row class of rec (CODEGEN.md §4.2, §5.9; DECISIONS 323).
func (g *gen) rowClass(rec *ir.Record) string {
	owner := g.owner()
	if rec.Pkg == owner {
		return g.typeName(rec)
	}
	return g.qualifier(owner) + g.pl.RowName(rec)
}

// owner is the package whose class layout storage describes: g.view, or this one.
func (g *gen) owner() string {
	if g.view != "" {
		return g.view
	}
	return g.p.Name
}

// tableHook is the hook of package holder building a row of its table of rec from the record, its id and retired flag: its entry hook for its own record, its row hook for another's (log-2026-10-06 "U1 review" 1).
func (g *gen) tableHook(holder string, rec *ir.Record) string {
	if rec.Pkg == holder {
		return g.pl.EntryHook(rec)
	}
	return g.pl.RowHook(rec)
}

// rowBase is the record part of row, a row class's base or the row itself.
func (g *gen) rowBase(row string, rec *ir.Record) string {
	if rec.Pkg == g.owner() {
		return row
	}
	return fmt.Sprintf(baseOfFormat, g.typeName(rec), row)
}

// rowIDType is the id a row class of another package's record holds: this package's id enum in a baked emit, unless a table field holds the rows, whose ids are strings in every mode (CODEGEN.md §5.3, §5.9).
func (g *gen) rowIDType(rec *ir.Record) string {
	if _, inField := g.rowFriends[rec]; g.baked() && !inField {
		return g.qualifier(g.p.Name) + g.pl.IDName(rec)
	}
	return cppString
}

// rowClassDecl is this package's row class of another package's record: derived publicly from it, so every getter of the record is reached and none sets anything, with its own id and retired flag (CODEGEN.md §5.9, §7.2).
func (g *gen) rowClassDecl(rec *ir.Record) {
	name := g.pl.RowName(rec)
	g.h.printf(rowOpenFormat, name, g.typeName(rec))
	idLine, idDecl := getIDLine, idMemberDecl
	if id := g.rowIDType(rec); id != cppString {
		idLine, idDecl = fmt.Sprintf(getterFormat, id, idGetter, "", fmt.Sprintf(returnFormat, idMember)), fmt.Sprintf(memberFormat, id, idMember, initBraces)
	}
	g.h.lineAt(1, idLine)
	g.h.lineAt(1, getRetiredLine)
	g.h.blank()
	g.h.line(privateLabel)
	g.h.linef(1, friendAccessFormat, g.pl.AccessName())
	g.h.linef(1, friendAccessFormat, g.pl.MakeStruct(g.p.Name))
	if !g.baked() {
		for _, n := range g.rowFriends[rec] {
			g.h.linef(1, friendDecodeFormat, n)
		}
	}
	g.h.blank()
	g.h.lineAt(1, idDecl)
	g.h.lineAt(1, retiredMemberDecl)
	g.h.line(closeClass)
	g.h.blank()
}

// rowHook is the member of detail::<P>Make building a row class from its record, id and retired flag (CODEGEN.md §5.14).
func (g *gen) rowHook(rec *ir.Record) hook {
	row, base := g.rowClass(rec), g.typeName(rec)
	return hook{
		ret: row, name: g.pl.RowHook(rec),
		params: []string{base + space + rowRecordParam, g.rowIDType(rec) + space + idKeyName, cppBool + space + rowRetiredParam},
		body: []string{
			fmt.Sprintf(assignFormat, fmt.Sprintf(baseOfFormat, base, outVar), fmt.Sprintf(moveFormat, rowRecordParam)),
			fmt.Sprintf(assignFormat, outPrefix+idMember, fmt.Sprintf(moveFormat, idKeyName)),
			fmt.Sprintf(assignFormat, outPrefix+retiredMember, rowRetiredParam),
		},
	}
}

// entryField is how a loader reads field f of an entry of rec: its member, or, for another package's record, whose members are its own, its getter.
func (g *gen) entryField(rec *ir.Record, f *ir.Field) string {
	if rec.Pkg != g.p.Name {
		return g.getterName(f) + callSuffix
	}
	m, err := g.member(f.Name)
	g.fail(err)
	return m
}

// valueElem is the class of a table's or keyed list's entries: a table's rows (CODEGEN.md §5.9).
func (g *gen) valueElem(v *ir.Value) string {
	if rec, ok := v.Type.Elem.Named.(*ir.Record); ok && v.Type.Kind == types.Table {
		return g.rowClass(rec)
	}
	return g.storage(*v.Type.Elem)
}
