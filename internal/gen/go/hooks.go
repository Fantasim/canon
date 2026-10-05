package gogen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// hookShape is the shape of text/hooks.txt's hook: a doc line, the signature, the statements before the return.
type hookShape struct {
	Name, Params, Type, Pre, Ret string
}

// hookParts is what a hook of this package's class is made of: its parameters, the members it sets from them, and the statements filling the entries its baked data resolves (CODEGEN.md §5.14).
type hookParts struct {
	params []member
	pairs  []pair
	stmts  strings.Builder
}

// hooks writes every make hook of the package, used or not (CODEGEN.md §5.14): records', cases' and case types', dependent branches', in declaration order, then its row types'.
func (g *gen) hooks() {
	for _, t := range g.p.Types {
		switch x := t.(type) {
		case *ir.Record:
			h := g.names.RecordHook(x)
			g.classHook(h.Name, g.bodyOf(x), h)
		case *ir.Variant:
			for _, c := range x.Cases {
				g.caseHooks(x, c)
			}
		case *ir.Dependent:
			for i := range x.Branches {
				g.branchHook(x, i)
			}
		}
	}
	for _, t := range g.p.Types {
		if rec, ok := t.(*ir.Record); ok && (g.tableOf[rec] != nil || g.names.NestedRow(rec)) {
			g.entryHook(rec)
		}
	}
	for _, row := range g.names.Rows() {
		g.rowHook(row.Record)
	}
}

// entryHook writes MakeEntry_<T>(record, id, retired) of a record a table of the package holds: another package fills such a table's rows through it (log-2026-10-06 "U1 review" 1).
func (g *gen) entryHook(rec *ir.Record) {
	name := g.goName(rec)
	params := []member{{rowRecordStore, name}, {ir.GoIDStore, g.idType(rec)}, {ir.GoRetiredStore, goBool}}
	pre := fmt.Sprintf(setRowFormat, rowRecordStore, ir.GoIDStore, ir.GoRetiredStore, ir.GoIDStore, ir.GoRetiredStore)
	g.writeHook(g.names.EntryHook(rec), name, params, pre, rowRecordStore)
}

// writeHook writes one hook through text/hooks.txt.
func (g *gen) writeHook(name, typ string, params []member, pre, ret string) {
	ps := make([]string, len(params))
	for i, p := range params {
		ps[i] = p.name + space + p.typ
	}
	g.exec(tmplHook, hookShape{Name: name, Params: strings.Join(ps, listSep), Type: typ, Pre: pre, Ret: ret})
}

// classHook writes Make_<T> or MakeCase_<V>_<Case>: T{…} of its parameters, after filling the entries the baked data resolves.
func (g *gen) classHook(name string, b *body, hook ir.GoHook) {
	defer g.enter(b.owner)()
	if !hook.Written { // a data loader resolves one of its refs: no hook, its name reserved (log-2026-10-06 "U1 review" 3)
		return
	}
	var h hookParts
	for _, hs := range hook.Slots {
		if hs.Finite != nil {
			g.finitePart(&h, b, hs.Fn)
			continue
		}
		if s := ownSlot(b, hs); s != nil {
			g.slotPart(&h, s)
		}
	}
	lit := compositeLit(b.goName, h.pairs)
	if h.stmts.Len() == 0 {
		g.writeHook(name, b.goName, h.params, "", lit)
		return
	}
	g.writeHook(name, b.goName, h.params, fmt.Sprintf(defineFormat, selfRecv, lit)+h.stmts.String(), selfRecv)
}

// ownSlot is the slot of b a hook slot fills: its field's or precomputed fn's.
func ownSlot(b *body, hs ir.GoHookSlot) *slot {
	for _, s := range b.slots {
		if hs.Field != nil && s.src == hs.Field || hs.Fn != nil && s.fn == hs.Fn {
			return s
		}
	}
	return nil
}

// slotPart takes a slot as the hook view lays it out, sets the members it shares with the slot, and resolves the entry a baked emit stores (CODEGEN.md §5.8, §5.14).
func (g *gen) slotPart(h *hookParts, s *slot) {
	ps := g.storage(viewSlot(s))
	own := g.storage(s)
	h.params = append(h.params, ps...)
	for _, p := range ps {
		if slices.ContainsFunc(own, func(m member) bool { return m.name == p.name }) {
			h.pairs = append(h.pairs, pair{p.name, p.name})
		}
	}
	if s.Resolved && !g.isData() {
		h.stmts.WriteString(g.resolveHook(s))
	}
}

// finitePart takes a lookup's cells: keys for a ref result, which a baked emit resolves into its entries and a data emit keeps in its key table.
func (g *gen) finitePart(h *hookParts, b *body, fn *ir.ExportFn) {
	i := slices.IndexFunc(b.finite, func(f *finiteMethod) bool { return f.fn == fn })
	if i < 0 {
		g.failf(ErrMalformed, "a hook of %s without its lookup %s", b.owner, fn.Name)
		return
	}
	f, view := b.finite[i], g.viewFinite(b.finite[i])
	h.params = append(h.params, g.finiteParams(view)...)
	switch {
	case f.res.Resolved:
		h.stmts.WriteString(g.cellsHook(f, view, g.resolveCell(f)))
	case view.split:
		h.stmts.WriteString(g.cellsHook(f, view, func(dst, src, ok string) string {
			return fmt.Sprintf(assignPairFormat, dst+pairValue, dst+pairOK, src, ok)
		}))
	default:
		h.pairs = append(h.pairs, pair{f.store, f.store})
	}
}

// hookFind is how a hook finds an entry of the package's baked data by key: Get on a table, Find on a keyed list.
func (g *gen) hookFind(r *ir.RefTarget) (find string, keyed bool) {
	info := g.byValue[r.Value]
	if info == nil {
		g.failf(ErrMalformed, "a resolved ref into %s, which the emit does not hold", r.Value)
		return nilLit, false
	}
	method := ir.GoGet
	if r.Keyed {
		method = ir.GoFind
	}
	return g.names.AccessorName(info.v) + callSuffix + dot + method, r.Keyed
}

// lookupLine sets dst to the entry key names.
func lookupLine(dst, find, key string, keyed bool) string {
	if keyed {
		return fmt.Sprintf(hookFindFormat, dst, find, key)
	}
	return fmt.Sprintf(hookLookupFormat, dst, find, key)
}

// listLines sets dst to the entries of the key list keys; its locals avoid keys' own name.
func (g *gen) listLines(dst, keys, find string, keyed bool, r *ir.RefTarget) string {
	root, _, _ := strings.Cut(keys, lbracket)
	entries, i := avoid(hookEntries, root), avoid(hookIndex, root)
	line := lookupLine(entries+lbracket+i+rbracket, find, keys+atCall+i+rparen, keyed)
	return fmt.Sprintf(hookListFormat, entries, g.entryType(r), keys, i, line, dst, g.rt())
}

// avoid is name with `_` added until it is none of taken.
func avoid(name string, taken ...string) string {
	for slices.Contains(taken, name) {
		name += underscore
	}
	return name
}

// resolveHook fills a resolved slot's entry, or entries, from the key parameter (CODEGEN.md §5.14).
func (g *gen) resolveHook(s *slot) string {
	find, keyed := g.hookFind(s.Ref)
	dst := selfDot + s.Store
	var b strings.Builder
	switch {
	case s.Optional:
		fmt.Fprintf(&b, ifOpenFormat, s.OKStore)
	case s.List:
		b.WriteString(lbrace + newline)
	}
	if s.List {
		b.WriteString(g.listLines(dst, s.KeyStore, find, keyed, s.Ref))
	} else {
		b.WriteString(lookupLine(dst, find, s.KeyStore, keyed))
	}
	if s.Optional || s.List {
		b.WriteString(closeBrace)
	}
	return b.String()
}

// cellsHook fills a lookup's table cell by cell in domain order from the hook's parameters: cell writes dst from the value src and, with split cells, the presence ok.
func (g *gen) cellsHook(f, view *finiteMethod, cell func(dst, src, ok string) string) string {
	src, dst, ok := view.store, selfDot+f.store, ""
	if view.split {
		ok = view.res.OKStore
	}
	var b strings.Builder
	for n := range f.dims {
		i := avoid(hookCellIndex+strconv.Itoa(n), view.store, ok)
		fmt.Fprintf(&b, rangeOpenFormat, i, dst)
		src, dst = src+lbracket+i+rbracket, dst+lbracket+i+rbracket
		if ok != "" {
			ok += lbracket + i + rbracket
		}
	}
	b.WriteString(cell(dst, src, ok))
	b.WriteString(strings.Repeat(closeBrace, len(f.dims)))
	return b.String()
}

// resolveCell resolves one key cell into its entry: a list's keys into its entries, none left nil or unmarked.
func (g *gen) resolveCell(f *finiteMethod) func(dst, src, ok string) string {
	find, keyed := g.hookFind(f.res.Ref)
	return func(dst, src, ok string) string {
		switch {
		case !f.res.Optional && !f.res.List:
			return lookupLine(dst, find, src, keyed)
		case !f.res.List:
			return fmt.Sprintf(ifOpenFormat, ok) + lookupLine(dst, find, src, keyed) + closeBrace
		case !f.res.Optional:
			return lbrace + newline + g.listLines(dst, src, find, keyed, f.res.Ref) + closeBrace
		}
		return fmt.Sprintf(ifOpenFormat, ok) + g.listLines(dst+pairValue, src, find, keyed, f.res.Ref) +
			fmt.Sprintf(markFormat, dst+pairOK) + closeBrace
	}
}

// caseHooks writes Make_<V>_<Case>, then MakeCase_<V>_<Case> for a case with fields, which the first calls (CODEGEN.md §5.14).
func (g *gen) caseHooks(v *ir.Variant, c *ir.Case) {
	h := g.names.CaseHook(v, c)
	name, kind := g.goName(v), pair{ir.GoKindStore, g.kindLit(v, slices.Index(v.Cases, c))}
	if len(c.Fields) == 0 {
		g.writeHook(h.Name, name, nil, "", compositeLit(name, []pair{kind}))
		return
	}
	b := g.caseBody(v, c)
	if !h.Written {
		return
	}
	params := g.ownHookParams(b, h.Slots)
	args := make([]string, len(params))
	for i, p := range params {
		args[i] = p.name
	}
	pre := fmt.Sprintf(defineFormat, selfRecv, h.CaseType+callArgs(args))
	g.writeHook(h.Name, name, params, pre, compositeLit(name, []pair{kind, {ir.GoCaseStore, ampersand + selfRecv}}))
	g.classHook(h.CaseType, b, h)
}

// ownHookParams are the parameters of a hook of this package's class.
func (g *gen) ownHookParams(b *body, slots []ir.GoHookSlot) []member {
	var h hookParts
	for _, hs := range slots {
		if hs.Finite != nil {
			g.finitePart(&h, b, hs.Fn)
		} else if s := ownSlot(b, hs); s != nil {
			g.slotPart(&h, s)
		}
	}
	return h.params
}

// branchHook writes Make_<D>_<Branch>: the value As<Branch> returns, and a define branch's value too (CODEGEN.md §5.6, §5.14).
func (g *gen) branchHook(d *ir.Dependent, i int) {
	defer g.enter(d.QName())()
	n, br := g.names.Dependent(d), d.Branches[i]
	if i >= len(n.Branches) {
		g.failf(ErrMalformed, "branch %d of %s, which the plan does not name", i, d.QName())
		return
	}
	b := n.Branches[i]
	params := []member{{n.ValueStore, g.goType(br.Type)}}
	parts := []pair{{n.BranchStore, b.Member}, {n.ValueStore, n.ValueStore}}
	if b.AsValue != "" {
		params = append(params, member{n.DefineStore, goInt64})
		parts = append(parts, pair{n.DefineStore, n.DefineStore})
	}
	g.writeHook(g.names.MakeBranchName(d, br), n.Type, params, "", compositeLit(n.Type, parts))
}

// rowHook writes Make_<Element>Row(record, id, retired) of this package's row type of another package's record (CODEGEN.md §5.9, §5.14).
func (g *gen) rowHook(rec *ir.Record) {
	row := g.names.Row(rec)
	params := []member{{rowRecordStore, g.typeName(rec)}, {ir.GoIDStore, row.ID}, {ir.GoRetiredStore, goBool}}
	parts := []pair{{rowRecordStore, rowRecordStore}, {ir.GoIDStore, ir.GoIDStore}, {ir.GoRetiredStore, ir.GoRetiredStore}}
	g.writeHook(g.names.RowHook(rec), row.Name, params, "", compositeLit(row.Name, parts))
}
