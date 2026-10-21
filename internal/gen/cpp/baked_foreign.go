package cppgen

import (
	"fmt"
	"strconv"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// foreignHook is the make hook of another package's class a baked literal calls: its package, name and members (CODEGEN.md §5.14).
type foreignHook struct {
	pkg, name string
	members   []ir.CppHookMember
}

// fillForeign writes another package's record or case through its owner's make hook, a literal member as the argument itself (no local for debug info to track), any other through a local of a block (CODEGEN.md §2.8 Literals, §5.14).
func (g *gen) fillForeign(a at, lhs string, c class, r *value.Record, h foreignHook) {
	prev := g.view
	g.view = h.pkg
	defer func() { g.view = prev }()
	in := a.in()
	prefix := fmt.Sprintf(bakedLocalFormat, in.depth)
	args, locals := g.foreignArgs(r, h.members, prefix)
	if len(locals) == 0 {
		a.line(assignFormat, lhs, g.makeCall(h.pkg, h.name, args))
		return
	}
	a.line(openBrace)
	for _, l := range g.hookLocalDecls(locals, prefix) {
		in.w.lineAt(in.depth, l)
	}
	fields, _ := c.shape()
	for _, m := range locals {
		if m.Field != nil {
			g.fillForeignField(in, prefix, fields, m, r)
			continue
		}
		g.fillForeignStored(in, prefix+m.Member, m.Fn, r)
	}
	in.line(assignFormat, lhs, g.makeCall(h.pkg, h.name, args))
	a.line(closeBrace)
}

// foreignArgs are the hook's arguments, a literal or a moved local, and the members held in locals.
func (g *gen) foreignArgs(r *value.Record, members []ir.CppHookMember, prefix string) (args []string, locals []ir.CppHookMember) {
	for _, m := range members {
		if text, ok := g.foreignArg(r, m); ok {
			args = append(args, text)
			continue
		}
		locals = append(locals, m)
		args = append(args, movedLocals([]ir.CppHookMember{m}, prefix)...)
	}
	return args, locals
}

// foreignArg is a hook member's argument when a literal writes it: a field's or a precomputed
// result's literal, or its type's default when absent; false for a member a local holds.
func (g *gen) foreignArg(r *value.Record, m ir.CppHookMember) (string, bool) {
	var t ir.TypeRef
	var v value.Value
	switch {
	case m.DefineMember != "":
		return "", false
	case m.Field != nil && !g.boxed[m.Field] && ir.HeldApp(m.Field.Type) == nil:
		t, v = m.Field.Type, g.fieldOf(r, m.Field.Name)
	case m.Fn != nil && m.Fn.Kind == ir.FnPrecomputed:
		t, _ = resultType(m.Fn.Result)
		v = g.instanceOf(m.Fn, r).Result
	default:
		return "", false
	}
	switch {
	case absent(v):
		return g.hookStorage(m) + initBraces, true
	case g.literalType(t):
		return g.literalOf(t, v), true
	}
	return "", false
}

// fillForeignField writes one field of another package's class into its local: boxed, a dependent value through its branch hook, a define ref's values beside its keys.
func (g *gen) fillForeignField(a at, prefix string, fields []*ir.Field, m ir.CppHookMember, r *value.Record) {
	f := m.Field
	leave := g.enter(g.at + qnameSep + f.Name)
	defer leave()
	v := g.fieldOf(r, f.Name)
	local := prefix + m.Member
	switch {
	case absent(v):
		return
	case g.boxed[f]:
		a.line(makeUniqueFormat, local, g.storage(f.Type))
		g.fillValue(a, fmt.Sprintf(derefFormat, local), f.Type, false, v)
	case ir.HeldApp(f.Type) != nil:
		g.fillDependent(a, local, fields, f, r)
	default:
		g.fillValue(a, local, f.Type, f.Optional, v)
	}
	if m.DefineMember != "" {
		g.fillDefineTo(a, prefix+m.DefineMember, f, v)
	}
}

// fillForeignStored writes a stored fn's result for receiver r into its local: one value, or a lookup's cells.
func (g *gen) fillForeignStored(a at, local string, fn *ir.ExportFn, r *value.Record) {
	in := g.instanceOf(fn, r)
	t, optional := resultType(fn.Result)
	if fn.Kind == ir.FnPrecomputed {
		g.fillValue(a, local, t, optional, in.Result)
		return
	}
	cells, ok := g.tableCells(fn, in.Table)
	if ok && g.hoistCells(a, local, t, optional, cells) {
		return
	}
	for i := 0; ok && i < len(cells); i++ {
		g.fillValue(a, fmt.Sprintf(indexFormat, local, strconv.Itoa(i)), t, optional, cells[i])
	}
}

// fillForeignCase writes a variant value of another package through its case's hook.
func (g *gen) fillForeignCase(a at, lhs string, v *ir.Variant, cs *ir.Case, r *value.Record) {
	h := g.written(g.pl.CaseHook(v, cs))
	if len(cs.Fields) == 0 {
		a.line(assignFormat, lhs, g.makeCall(v.Pkg, h.Name, nil))
		return
	}
	g.fillForeign(a, lhs, class{variant: v, cs: cs}, r, foreignHook{v.Pkg, h.Name, h.Members})
}

// fillRow writes a row of this package's row class of another package's record: the record into its base through its owner's hook, then the row's own id and retired flag (CODEGEN.md §5.9).
func (g *gen) fillRow(a at, local string, rec *ir.Record, r *value.Record, id string) {
	h := g.written(g.pl.RecordHook(rec))
	g.fillForeign(a, fmt.Sprintf(baseOfFormat, g.typeName(rec), local), class{rec: rec}, r, foreignHook{rec.Pkg, h.Name, h.Members})
	a.line(assignFormat, local+memberAccess+idMember, id)
	if r.Ident != nil && r.Ident.Retired {
		a.line(assignFormat, local+memberAccess+retiredMember, strconv.FormatBool(true))
	}
}

// fillHeldRow writes a row of a table another package's class holds: the record through its owner's hook, then the row through its holder's entry or row hook (log-2026-10-06 "U1 review" 1).
func (g *gen) fillHeldRow(a at, local string, rec *ir.Record, r *value.Record, id string) {
	holder, base := g.owner(), g.rowBase(local, rec)
	h := g.written(g.pl.RecordHook(rec))
	g.fillForeign(a, base, class{rec: rec}, r, foreignHook{rec.Pkg, h.Name, h.Members})
	retired := r.Ident != nil && r.Ident.Retired
	args := []string{fmt.Sprintf(moveFormat, base), id, strconv.FormatBool(retired)}
	a.line(assignFormat, local, g.makeCall(holder, g.tableHook(holder, rec), args))
}

// rowIDLit is a table value's row id as its row class holds it: a member of this package's id enum, or its key.
func (g *gen) rowIDLit(rec *ir.Record, r *value.Record) string {
	if r.Ident == nil {
		g.malformed(bakedEntryKey, g.at)
		return cppInvalid
	}
	if id := g.rowIDType(rec); id != cppString {
		return id + scopeSep + g.pl.IDMember(r.Ident.Key.S)
	}
	return stringLit(r.Ident.Key.S)
}

// hookLocalDecls declare a local per hook member, named prefix + its member, initialized as the member is (CODEGEN.md §7.2).
func (g *gen) hookLocalDecls(members []ir.CppHookMember, prefix string) []string {
	var out []string
	for _, m := range members {
		init := ""
		switch {
		case m.Field != nil && !g.boxed[m.Field]:
			init = g.memberInit(m.Field.Type, m.Field.Optional)
		case m.Fn != nil && m.Fn.Kind == ir.FnLookup:
			init = initBraces
		case m.Fn != nil:
			t, optional := resultType(m.Fn.Result)
			init = g.memberInit(t, optional)
		}
		out = append(out, fmt.Sprintf(memberFormat, g.hookStorage(m), prefix+m.Member, init))
		if m.DefineMember != "" {
			t := defineValueType(m.Field.Type)
			out = append(out, fmt.Sprintf(memberFormat, g.memberType(t, m.Field.Optional), prefix+m.DefineMember, g.memberInit(t, m.Field.Optional)))
		}
	}
	return out
}
