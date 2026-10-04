package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// reader is a function of a baked emit that reads Data: its result type, name, parameters, the lines computing a cell, the return of a member and its doc.
type reader struct {
	typ, name, params, doc string
	prelude                []string
	body                   string
}

// readForm is what an accessor or a stored package fn returns and how it returns member m: a resolved ref's entries when target is set, else what a field getter of t returns (CODEGEN.md §5.8, §5.9).
func (g *gen) readForm(t ir.TypeRef, optional bool, target *ir.Value, m string) (typ, body string) {
	if target == nil {
		if optional && !g.byValue(t) {
			return g.getterType(t, optional), fmt.Sprintf(returnPtrFormat, m)
		}
		return g.getterType(t, optional), fmt.Sprintf(returnFormat, m)
	}
	list, _ := refSlot(t)
	elem := g.valueElem(target)
	vec := fmt.Sprintf(vectorFormat, fmt.Sprintf(constPtrFormat, elem))
	switch {
	case list && optional:
		return fmt.Sprintf(constPtrFormat, vec), fmt.Sprintf(returnPtrFormat, m)
	case list:
		return fmt.Sprintf(constRefFormat, vec), fmt.Sprintf(returnFormat, m)
	case optional:
		return fmt.Sprintf(constPtrFormat, elem), fmt.Sprintf(returnFormat, m)
	}
	return fmt.Sprintf(constRefFormat, elem), fmt.Sprintf(derefReturnFormat, m)
}

// bakedTarget is the container a ref of a baked emit resolves into, or nil for a key only (CODEGEN.md §5.8).
func (g *gen) bakedTarget(t ir.TypeRef) *ir.Value {
	if _, isRef := refSlot(t); !isRef || !g.pl.Resolves(t, nil) {
		return nil
	}
	if t.Kind == types.List {
		t = *t.Elem
	}
	return g.valueNamed(t.Ref.Value)
}

// storedKey is the key a call of a stored fn that is not constexpr returns to a translated body: a resolved keyed-list entry's key getter, else the key itself (ir's CallFn holds only scalar or ref results).
func (g *gen) storedKey(t ir.TypeRef, call string) string {
	target := g.bakedTarget(t)
	if target == nil || target.Type.KeyedBy == nil {
		return call
	}
	kf := g.keyField(target.Type)
	if kf == nil {
		return cppInvalid
	}
	return call + memberAccess + g.getterName(kf) + callSuffix
}

// dataExpr is member m of the baked data, read through detail::<P>Access::Get().
func (g *gen) dataExpr(m string) string {
	return fmt.Sprintf(dataGetFormat, g.pl.AccessName(), m)
}

// valueReaders are a value's accessors: Get<V>, and Get<V>Key(s) beside a resolved ref (CODEGEN.md §5.9).
func (g *gen) valueReaders(v *ir.Value) []reader {
	getter, resolved := g.pl.Accessor(v)
	m := g.dataExpr(g.pl.DataMember(v.Name))
	if ir.IsContainer(v) {
		return []reader{{typ: fmt.Sprintf(constRefFormat, g.pl.ContainerName(v)), name: getter, doc: v.Doc, body: fmt.Sprintf(returnFormat, m)}}
	}
	t, optional := resultType(v.Type)
	var out []reader
	doc := v.Doc
	if target := g.bakedTarget(t); target != nil {
		typ, body := g.readForm(t, optional, target, g.dataExpr(g.pl.RefData(v.Name)))
		out = append(out, reader{typ: typ, name: resolved, doc: doc, body: body})
		doc = ""
	}
	typ, body := g.readForm(t, optional, nil, m)
	return append(out, reader{typ: typ, name: getter, doc: doc, body: body})
}

// accessors declare every value's accessors after the containers (CODEGEN.md §2.7 step 5, §5.9).
func (g *gen) accessors() {
	if !g.baked() || len(g.values) == 0 {
		return
	}
	for _, v := range g.values {
		leave := g.enter(v.Name)
		for _, r := range g.valueReaders(v) {
			g.doc(0, r.doc)
			g.h.printf(declFormat, fmt.Sprintf(signatureFormat, r.typ, r.name, ""))
		}
		leave()
	}
	g.h.blank()
}

// fnReader is a stored package fn as a function of the baked data: its cell, read like a getter (CODEGEN.md §5.10).
func (g *gen) fnReader(fn *ir.ExportFn) reader {
	doms := g.fnDomains(fn)
	acc := g.lookupAccess(fn, doms)
	t, optional := resultType(fn.Result)
	target := g.bakedTarget(t)
	typ, body := g.readForm(t, optional, target, acc.cell(g.dataExpr(g.pl.DataMember(fn.Name))))
	return reader{typ: typ, name: g.pl.FnName(fn), params: acc.params, doc: fn.Doc, prelude: acc.prelude, body: body}
}

// fnProto is a constexpr fn's prototype, which a translated body calling it needs first.
func (g *gen) fnProto(fn *ir.ExportFn) reader {
	acc := g.lookupAccess(fn, g.fnDomains(fn))
	return reader{typ: constexprPrefix + g.cellType(fn.Result), name: g.pl.FnName(fn), params: acc.params}
}

// cellType is a constexpr fn's result and cell type: a String is a std::string_view (decision 293).
func (g *gen) cellType(t ir.TypeRef) string {
	if t.Kind == types.String || t.Kind == types.LitUnion {
		return cppStringView
	}
	return g.storage(t)
}

// constexprFn writes a stored package fn with a constexpr scalar result in the header: its cells in detail, then the function (CODEGEN.md §5.10, decision 293).
func (g *gen) constexprFn(fn *ir.ExportFn) {
	doms := g.fnDomains(fn)
	cells := []value.Value{fn.Value}
	if fn.Kind == ir.FnLookup {
		var ok bool
		if cells, ok = g.tableCells(fn, fn.Table); !ok {
			return
		}
	}
	items := make([]string, len(cells))
	for i, c := range cells {
		items[i] = g.element(fn.Result, c)
	}
	typ := g.cellType(fn.Result)
	name := g.pl.CellsName(fn)
	g.h.line(detailOpen)
	g.h.printf(cellsFormat, typ, len(cells), name, braced(items))
	g.h.line(detailClose)
	g.h.blank()
	acc := g.lookupAccess(fn, doms)
	cell := acc.cell(detailPrefix + name)
	if doms == nil {
		cell = fmt.Sprintf(indexFormat, detailPrefix+name, zeroInt)
	}
	g.doc(0, fn.Doc)
	g.writeFunc(reader{typ: constexprPrefix + typ, name: g.pl.FnName(fn), params: acc.params, prelude: acc.prelude, body: fmt.Sprintf(returnFormat, cell)}, &g.h)
}

// braced is {a, b} on one line, or one item per line past maxInline bytes.
func braced(items []string) string {
	inline := strings.Join(items, listSep)
	if len(inline) <= maxInline {
		return openBrace + inline + closeBrace
	}
	return openBrace + newline + indentUnit + strings.Join(items, comma+newline+indentUnit) + comma + newline + closeBrace
}

// writeFunc writes a free function: one line, or a block when its cell needs computing first.
func (g *gen) writeFunc(r reader, w *writer) {
	sig := fmt.Sprintf(signatureFormat, r.typ, r.name, r.params)
	if len(r.prelude) == 0 {
		w.printf(funcLineFormat, sig, r.body)
		return
	}
	w.printf(funcOpenFormat, sig)
	for _, l := range append(r.prelude, r.body) {
		w.lineAt(1, l)
	}
	w.line(closeBrace)
}

// storedFnDecl declares a stored package fn the source defines (CODEGEN.md §7.1).
func (g *gen) storedFnDecl(fn *ir.ExportFn) {
	r := g.fnReader(fn)
	g.doc(0, r.doc)
	g.h.printf(declFormat, fmt.Sprintf(signatureFormat, r.typ, r.name, r.params))
}

// bakedDefinitions define, after detail, the id enums' <Rec>IdFromWire, the accessors, then the stored package fns that are not constexpr (CODEGEN.md §5.3, §5.9, §5.10, §7.1).
func (g *gen) bakedDefinitions() {
	g.idFromWires()
	for _, v := range g.values {
		leave := g.enter(v.Name)
		for _, r := range g.valueReaders(v) {
			g.writeFunc(r, &g.c)
		}
		leave()
	}
	if len(g.values) > 0 {
		g.c.blank()
	}
	for _, fn := range g.p.Fns {
		if fn.Kind != ir.FnTranslated && !g.pl.Constexpr(fn) {
			leave := g.enter(fn.Name)
			g.writeFunc(g.fnReader(fn), &g.c)
			g.c.blank()
			leave()
		}
	}
}
