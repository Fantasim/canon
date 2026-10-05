package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// fnMember writes a method and returns the members a stored one reads (CODEGEN.md §5.4, §5.10).
func (g *gen) fnMember(sc *scope, c class, fields []*ir.Field, fn *ir.ExportFn) []string {
	leave := g.enter(g.at + qnameSep + fn.Name)
	defer leave()
	name := g.pl.FnName(fn)
	g.h.blank()
	switch fn.Kind {
	case ir.FnPrecomputed:
		return g.storedGetter(sc, c, name, fn, nil)
	case ir.FnLookup:
		return g.storedGetter(sc, c, name, fn, g.domains(fn))
	default:
		g.doc(1, fn.Doc)
		g.fail(sc.add(name, fn.Name))
		g.h.lineAt(1, g.publicMethod(g.className(c), fields, fn, name))
		return nil
	}
}

// access is how a getter reaches its cell: parameters, the lines computing the cell, the cell.
type access struct {
	params  string
	prelude []string
	cell    func(member string) string
}

// storedGetter is a precomputed or lookup getter; a ref result gets <Name>Key() (log-2026-09-24).
func (g *gen) storedGetter(sc *scope, c class, name string, fn *ir.ExportFn, doms []domain) []string {
	m, err := g.member(fn.Name)
	g.fail(err)
	t, optional := resultType(fn.Result)
	acc := g.lookupAccess(fn, doms)
	var decls []string
	keyName, _ := g.pl.StoredGetter(fn)
	doc := fn.Doc
	if list, isRef := refSlot(t); isRef {
		s := slot{member: m, ref: g.pl.RefMember(fn.Name), wire: dollar + fn.Name, t: t, optional: optional, list: list, cells: cellCount(doms), paths: cellPaths(fn.Name, doms)}
		if target := g.resolvedTarget(s.target(), c); target != nil {
			g.doc(1, doc)
			doc = ""
			decls = append(decls, g.resolvedGetter(sc, name, acc, s, target))
			g.noteSlot(c, s, target)
		}
	}
	g.doc(1, doc)
	body := fmt.Sprintf(returnFormat, acc.cell(m))
	if optional && !g.byValue(t) {
		body = fmt.Sprintf(returnPtrFormat, acc.cell(m))
	}
	g.fail(sc.add(keyName, fn.Name))
	g.fail(sc.add(m, fn.Name))
	g.writeGetter(g.getterType(t, optional), keyName, acc, body)
	storage, init := g.memberType(t, optional), g.memberInit(t, optional)
	if doms != nil {
		storage, init = fmt.Sprintf(arrayFormat, storage, cellCount(doms)), initBraces
	}
	return append([]string{fmt.Sprintf(memberFormat, storage, m, init)}, decls...)
}

// writeGetter writes a one-line getter, or a block when the cell needs computing first.
func (g *gen) writeGetter(typ, name string, acc access, body string) {
	if len(acc.prelude) == 0 {
		g.h.linef(1, getterFormat, typ, name, acc.params, body)
		return
	}
	g.h.linef(1, getterOpenFormat, typ, name, acc.params)
	for _, l := range append(acc.prelude, body) {
		g.h.lineAt(depthTwo, l)
	}
	g.h.linef(1, closeBrace)
}

func cellCount(doms []domain) int {
	if doms == nil {
		return 0
	}
	return cells(doms)
}

// lookupAccess is a lookup's parameters and row-major cell; a non-member aborts (log-2026-09-24).
func (g *gen) lookupAccess(fn *ir.ExportFn, doms []domain) access {
	if doms == nil {
		return access{cell: plain}
	}
	var params, lines []string
	index := ""
	for i, d := range doms {
		p := fn.Params[i]
		arg := verbatim(p.Name)
		params = append(params, g.storage(p.Type)+space+arg)
		ord := freeName(fmt.Sprintf(indexVarFormat, i), fn)
		lines = append(lines, g.ordinalLines(d, arg, ord)...)
		index = rowMajor(index, len(d.keys), ord)
	}
	return access{
		params: strings.Join(params, listSep), prelude: lines,
		cell: func(m string) string { return fmt.Sprintf(indexFormat, m, index) },
	}
}

// ordinalLines compute an argument's position in its domain: a Bool's value, an enum's or a
// table id's index, a @codes enum's member by a switch over its codes.
func (g *gen) ordinalLines(d domain, arg, ord string) []string {
	switch {
	case d.enum == nil && !d.id:
		return []string{fmt.Sprintf(ordinalFormat, ord, arg)}
	case len(d.keys) == 0: // an empty domain holds no argument; the abort stays conditional, so a constexpr lookup stays valid (CODEGEN.md §5.10)
		return []string{fmt.Sprintf(ordinalFormat, ord, arg), fmt.Sprintf(emptyCheckFormat, ord)}
	case d.id || d.enum.Codes == nil:
		return []string{fmt.Sprintf(ordinalFormat, ord, arg), fmt.Sprintf(ordinalCheckFormat, ord, len(d.keys))}
	}
	lines := []string{fmt.Sprintf(ordinalVarDecl, ord), fmt.Sprintf(switchFormat, arg)}
	for i, m := range d.enum.Members {
		lines = append(lines, fmt.Sprintf(ordinalCaseFormat, g.typeName(d.enum)+scopeSep+g.pl.Enumerator(m), ord, i))
	}
	return append(lines, abortDefault, closeBrace)
}

// freeName is name, with `_` added until no parameter of fn is called so.
func freeName(name string, fn *ir.ExportFn) string {
	for {
		taken := false
		for _, p := range fn.Params {
			taken = taken || verbatim(p.Name) == name
		}
		if !taken {
			return name
		}
		name += underscore
	}
}
