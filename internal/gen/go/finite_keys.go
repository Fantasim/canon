package gogen

import (
	"fmt"
	"slices"
)

// finiteHead is what every function reading one finite table repeats: its receiver or
// `func`, its parameters, and the lines computing the index.
type finiteHead struct {
	prefix, params, prelude string
}

func (g *gen) writeFiniteFunc(h finiteHead, name, doc, result, body string) {
	g.body.WriteString(docFor(name, doc))
	g.printf(finiteFuncFormat, h.prefix, name, h.params, result, h.prelude, body)
}

// pairBody returns a cell, or its v and ok when the cells are {v, ok} pairs.
func pairBody(cell string, pair bool) string {
	if pair {
		return returnKw + cell + pairValue + listSep + cell + pairOK
	}
	return returnKw + cell
}

// writeRefFinite writes a ref-result method's entry getter when it resolves, and always its key getter <Name>ID(s)(params), which has the doc when alone (CODEGEN.md §2.6).
func (g *gen) writeRefFinite(h finiteHead, f *finiteMethod, cell, index string) {
	if !f.res.hasMain() {
		g.writeFiniteFunc(h, f.res.KeyGetter, f.fn.Doc, results(g.cellType(f), f.pair), pairBody(cell, f.pair))
		return
	}
	g.writeFiniteFunc(h, f.name, f.fn.Doc, results(g.cellType(f), f.pair), pairBody(cell, f.pair))
	result := results(g.slotKeyType(f.res), f.res.Optional)
	if g.isData() {
		keys := selfDot + f.res.KeyStore + index
		g.writeFiniteFunc(h, f.res.KeyGetter, "", result, pairBody(keys, f.res.Optional))
		return
	}
	g.writeFiniteFunc(h, f.res.KeyGetter, "", result, g.derivedCellKey(f, cell))
}

// derivedCellKey reads a baked cell's key off its entries, as a resolved field's key getter
// does (derivedKey): baked tables hold entries only.
func (g *gen) derivedCellKey(f *finiteMethod, cell string) string {
	tk := g.targetKey(f.res.Ref)
	if !f.res.List {
		if !f.res.Optional {
			return returnKw + cell + dot + tk
		}
		return ifNilFormat(cell, g.zeroKey(f.res.refType())) + returnKw + cell + dot + tk + listSep + trueLit
	}
	list, ok := cell, ""
	if f.pair {
		list, ok = cell+pairValue, listSep+cell+pairOK
	}
	keys, i := f.fresh(localKeys), f.fresh(localIndex)
	return fmt.Sprintf(cellKeysFormat, keys, g.goType(f.res.refType()), list, i, tk, g.rt(), ok)
}

// fresh is base, with `_` added until no parameter or index local of the method is called so.
func (f *finiteMethod) fresh(base string) string {
	for slices.Contains(f.params, base) || slices.Contains(f.indexes, base) {
		base += underscore
	}
	return base
}
