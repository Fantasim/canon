package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// hook is one static member of detail::<P>Make (CODEGEN.md §5.14): the value it returns, its parameters, the statements filling `out`, and late ones, which fill the resolved getters from the package's baked data, so the hook is defined after the value accessors.
type hook struct {
	ret, name    string
	params, body []string
	late         []string
}

// makeStructDef is detail::<P>Make, after the classes, one hook per record, case, case type, dependent branch and row class, every type written from the global namespace (CODEGEN.md §2.7 step 5, §5.14); it is written even when it has no member.
func (g *gen) makeStructDef() {
	hooks := g.ownHooks()
	g.h.line(detailOpen)
	g.h.printf(structOpenFormat, g.pl.MakeStruct(g.p.Name))
	for i, h := range hooks {
		if i > 0 {
			g.h.blank()
		}
		sig := fmt.Sprintf(signatureFormat, staticPrefix+h.ret, h.name, strings.Join(h.params, listSep))
		if len(h.late) > 0 {
			g.h.linef(1, declFormatLine, sig)
			continue
		}
		g.writeHookBody(1, sig, h, nil)
	}
	g.h.line(closeClass)
	g.h.line(detailClose)
	g.h.blank()
}

// lateHooks define, after the value accessors, the hooks that fill resolved getters from them (CODEGEN.md §5.14).
func (g *gen) lateHooks() {
	var late []hook
	for _, h := range g.ownHooks() {
		if len(h.late) > 0 {
			late = append(late, h)
		}
	}
	if len(late) == 0 {
		return
	}
	g.h.line(detailOpen)
	g.h.blank()
	for _, h := range late {
		name := g.pl.MakeStruct(g.p.Name) + scopeSep + h.name
		g.writeHookBody(0, fmt.Sprintf(signatureFormat, inlinePrefix+h.ret, name, strings.Join(h.params, listSep)), h, h.late)
		g.h.blank()
	}
	g.h.line(detailClose)
	g.h.blank()
}

// writeHookBody writes a hook's definition at depth: `out` default-constructed, filled, returned.
func (g *gen) writeHookBody(depth int, sig string, h hook, late []string) {
	g.h.linef(depth, funcOpenLine, sig)
	g.h.linef(depth+1, localFormat, h.ret, outVar, "")
	for _, l := range append(append([]string(nil), h.body...), late...) {
		g.h.lineAt(depth+1, l)
	}
	g.h.linef(depth+1, returnFormat, outVar)
	g.h.linef(depth, closeBrace)
}

// ownHooks are the package's hooks in declaration order, then its row classes' (CODEGEN.md §5.14).
func (g *gen) ownHooks() []hook {
	prevQ, prevAt := g.qualify, g.at
	g.qualify = true
	defer func() { g.qualify, g.at = prevQ, prevAt }()
	var out []hook
	for _, t := range g.p.Types {
		switch x := t.(type) {
		case *ir.Record:
			g.at = x.Name
			if h := g.pl.RecordHook(x); h.Written {
				out = append(out, g.recordHook(x, h))
			}
		case *ir.Variant:
			g.at = x.Name
			out = append(out, g.caseHooks(x)...)
		case *ir.Dependent:
			g.at = x.Name
			out = append(out, g.branchHooks(x)...)
		}
	}
	for _, t := range g.p.Types {
		if rec, ok := t.(*ir.Record); ok && g.entries[rec] {
			out = append(out, g.entryHook(rec))
		}
	}
	for _, row := range g.pl.Rows() {
		out = append(out, g.rowHook(row.Record))
	}
	return out
}

// recordHook builds a record from its storage members, in member order; a data or types-mode class whose loader resolves a ref has none (log-2026-10-06 "U1 review" 3).
func (g *gen) recordHook(rec *ir.Record, h ir.CppHook) hook {
	out := hook{ret: g.typeName(rec), name: h.Name}
	out.params, out.body, out.late = g.hookMembers(h.Members)
	return out
}

// entryHook builds an entry of one of the package's tables from its record, id and retired flag, for another package building a record holding such a table (log-2026-10-06 "U1 review" 1).
func (g *gen) entryHook(rec *ir.Record) hook {
	id := cppString
	if g.baked() && !g.pl.NestedRow(rec) {
		id = g.qualifier(g.p.Name) + g.pl.IDName(rec)
	}
	t := g.typeName(rec)
	return hook{
		ret: t, name: g.pl.EntryHook(rec),
		params: []string{t + space + rowRecordParam, id + space + idKeyName, cppBool + space + rowRetiredParam},
		body: []string{
			fmt.Sprintf(assignFormat, outVar, fmt.Sprintf(moveFormat, rowRecordParam)),
			fmt.Sprintf(assignFormat, outPrefix+idMember, fmt.Sprintf(moveFormat, idKeyName)),
			fmt.Sprintf(assignFormat, outPrefix+retiredMember, rowRetiredParam),
		},
	}
}

// caseHooks are a variant's case hooks, each followed by its case-type hook when the case has fields: the case hook builds the case type through it.
func (g *gen) caseHooks(v *ir.Variant) []hook {
	var out []hook
	for i, cs := range v.Cases {
		h := g.pl.CaseHook(v, cs)
		if !h.Written {
			continue
		}
		vh := hook{ret: g.typeName(v), name: h.Name}
		if h.CaseType == "" {
			vh.body = []string{fmt.Sprintf(emplaceIndexFormat, i)}
			out = append(out, vh)
			continue
		}
		ct := hook{ret: g.caseName(v, cs), name: h.CaseType}
		ct.params, ct.body, ct.late = g.hookMembers(h.Members)
		vh.params = ct.params
		vh.body = []string{fmt.Sprintf(emplaceHookFormat, i, h.CaseType, strings.Join(movedLocals(h.Members, ""), listSep))}
		out = append(out, vh, ct)
	}
	return out
}

// branchHooks are a dependent type's hooks, one per branch: its value, and a define branch's value (CODEGEN.md §5.6, §5.14).
func (g *gen) branchHooks(d *ir.Dependent) []hook {
	n := g.pl.Dependent(d)
	var out []hook
	for i, b := range d.Branches {
		h := hook{ret: g.typeName(d), name: g.pl.MakeBranchName(d, b)}
		h.params = []string{g.storage(b.Type) + space + ir.CppVariantMember}
		h.body = []string{fmt.Sprintf(emplaceMoveFormat, i, ir.CppVariantMember)}
		if n.Branches[i].AsValue != "" {
			h.params = append(h.params, cppInt64+space+n.DefineValue)
			h.body = append(h.body, fmt.Sprintf(assignFormat, outPrefix+n.DefineValue, n.DefineValue))
		}
		out = append(out, h)
	}
	return out
}

// hookMembers are a class's hook parameters, each moved into its member, and the late statements filling its resolved getters when the package is baked (CODEGEN.md §5.14).
func (g *gen) hookMembers(members []ir.CppHookMember) (params, body, late []string) {
	for _, m := range members {
		params = append(params, g.hookStorage(m)+space+m.Member)
		body = append(body, fmt.Sprintf(assignFormat, outPrefix+m.Member, fmt.Sprintf(moveFormat, m.Member)))
		if m.DefineMember != "" {
			params = append(params, g.memberType(defineValueType(m.Field.Type), m.Field.Optional)+space+m.DefineMember)
			body = append(body, fmt.Sprintf(assignFormat, outPrefix+m.DefineMember, fmt.Sprintf(moveFormat, m.DefineMember)))
		}
		if m.Resolved && g.baked() {
			late = append(late, g.hookResolve(m)...)
		}
	}
	return params, body, late
}

// hookStorage is the storage of a hook member as its class holds it (CODEGEN.md §7.2): a field's, boxed when its class reaches itself through it, or a stored fn's result, one per cell of a lookup.
func (g *gen) hookStorage(m ir.CppHookMember) string {
	if f := m.Field; f != nil {
		if g.boxed[f] {
			return fmt.Sprintf(uniquePtrFormat, g.storage(f.Type))
		}
		return g.memberType(f.Type, f.Optional)
	}
	t, optional := resultType(m.Fn.Result)
	storage := g.memberType(t, optional)
	if m.Fn.Kind == ir.FnLookup {
		storage = fmt.Sprintf(arrayFormat, storage, cellCount(g.domains(m.Fn)))
	}
	return storage
}

// slotOf is a hook member's slot, its type, optional flag and cells (0 but for a lookup), and its Canon name.
func (g *gen) slotOf(m ir.CppHookMember) (slot, string) {
	if m.Field != nil {
		return slot{t: m.Field.Type, optional: m.Field.Optional}, m.Field.Name
	}
	t, optional := resultType(m.Fn.Result)
	s := slot{t: t, optional: optional}
	if m.Fn.Kind == ir.FnLookup {
		s.cells = cellCount(g.domains(m.Fn))
	}
	return s, m.Fn.Name
}

// emptyLookup reports a lookup member whose domain is empty: it holds no cell.
func emptyLookup(m ir.CppHookMember, s slot) bool {
	return m.Fn != nil && m.Fn.Kind == ir.FnLookup && s.cells == 0
}

// hookResolve fills a resolved getter's entries from the key the hook took, looking it up in the package's baked container as Get or Find does (CODEGEN.md §5.8, §5.14).
func (g *gen) hookResolve(m ir.CppHookMember) []string {
	s, name := g.slotOf(m)
	t, optional, cells := s.t, s.optional, s.cells
	if emptyLookup(m, s) {
		return nil // no cell holds a key (CODEGEN.md §5.10)
	}
	list, _ := refSlot(t)
	target := t
	if list {
		target = *t.Elem
	}
	key, ref := outPrefix+m.Member, outPrefix+g.pl.RefMember(name)
	if cells > 0 {
		key, ref = fmt.Sprintf(indexFormat, key, cellLoopVar), fmt.Sprintf(indexFormat, ref, cellLoopVar)
	}
	lines := g.resolveLines(g.valueNamed(target.Ref.Value), key, ref, list, optional)
	if cells == 0 {
		return lines
	}
	out := []string{fmt.Sprintf(countForFormat, cellLoopVar, cellLoopVar, cells, cellLoopVar)}
	for _, l := range lines {
		out = append(out, indentUnit+l)
	}
	return append(out, closeBrace)
}

// resolveLines point ref at the entries of v the keys of key name: one, through a present optional, or each of a list.
func (g *gen) resolveLines(v *ir.Value, key, ref string, list, optional bool) []string {
	if v == nil {
		g.malformed(bakedEntryKey, g.at)
		return nil
	}
	_, accessor := g.pl.Accessor(v)
	find := hookFindFormat
	if v.Type.Kind == types.Table {
		find = hookGetFormat
	}
	at := func(k string) string { return fmt.Sprintf(find, g.own()+accessor, k) }
	switch {
	case list && optional:
		return []string{
			fmt.Sprintf(ifOpenFormat, key),
			indentUnit + fmt.Sprintf(emplaceFormat, refsVar, ref),
			indentUnit + fmt.Sprintf(hookEachFormat, derefStar+key, refsVar, at(hookKeyVar)),
			closeBrace,
		}
	case list:
		return []string{fmt.Sprintf(hookEachFormat, key, ref, at(hookKeyVar))}
	case optional:
		return []string{fmt.Sprintf(hookIfFormat, key, ref, at(derefStar+key))}
	}
	return []string{fmt.Sprintf(assignFormat, ref, at(key))}
}

// makeCall is a call of hook name of package pkg's detail::<P>Make, from the global namespace (CODEGEN.md §2.8).
func (g *gen) makeCall(pkg, name string, args []string) string {
	return fmt.Sprintf(makeCallFormat, g.qualifier(pkg)+detailPrefix+g.pl.MakeStruct(pkg), name, strings.Join(args, listSep))
}
