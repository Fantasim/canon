package cppgen

import (
	"fmt"
	"strconv"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// fillColumns writes one loop over a container's rows of this package's record filling, from an
// array each, the members arrays can hold, and returns them: fields, stored fns, and rec for the
// ids; the per-row blocks write the rest (DECISIONS 392).
func (g *gen) fillColumns(w *writer, v *ir.Value, rec *ir.Record, rows []*value.Record) map[any]bool {
	if rec.Pkg != g.p.Name || !g.hoists(len(rows)) {
		return nil
	}
	done := map[any]bool{}
	row := rowVar(depthTwo)
	stmts := g.idColumns(row, rec, rows, done)
	fields, fns := class{rec: rec}.shape()
	for _, f := range fields {
		if s, ok := g.fieldColumn(row, f, rows); ok {
			stmts, done[f] = append(stmts, s...), true
		}
	}
	for _, fn := range fns {
		if s, ok := g.storedColumn(row, fn, rows); ok {
			stmts, done[fn] = append(stmts, s...), true
		}
	}
	if len(done) == 0 {
		return nil
	}
	if len(stmts) == 0 { // every column held only empty lists: the rows stay as constructed
		return done
	}
	w.linef(1, countForFormat, indexLocal, indexLocal, len(rows), indexLocal)
	at := fmt.Sprintf(keyedElemFormat, g.rowClass(rec), dataPrefix+g.pl.DataMember(v.Name)+rowsMember, indexLocal)
	w.linef(depthTwo, rowAliasFormat, row, at)
	for _, s := range stmts {
		w.lineAt(depthTwo, s)
	}
	w.linef(1, closeBrace)
	return done
}

// idColumns are the rows' ids and retired flags of a table's rows, as fillRecord writes them.
func (g *gen) idColumns(row string, rec *ir.Record, rows []*value.Record, done map[any]bool) []string {
	if !g.entries[rec] {
		return nil
	}
	ids, retired, anyRetired := make([]string, len(rows)), make([]string, len(rows)), false
	for i, r := range rows {
		if !g.rowOfTable(rec, r) {
			return nil
		}
		ids[i] = g.pl.IDName(rec) + scopeSep + g.pl.IDMember(r.Ident.Key.S)
		retired[i] = strconv.FormatBool(r.Ident.Retired)
		anyRetired = anyRetired || r.Ident.Retired
	}
	done[rec] = true
	out := []string{fmt.Sprintf(assignFormat, row+memberAccess+idMember, columnAt(g.pool.array(g.pl.IDName(rec), ids)))}
	if anyRetired {
		out = append(out, fmt.Sprintf(assignFormat, row+memberAccess+retiredMember, columnAt(g.pool.array(cppBool, retired))))
	}
	return out
}

// columnAt is row i's element of array name.
func columnAt(name string) string { return fmt.Sprintf(indexFormat, name, indexLocal) }

// fieldColumn is field f of every row from arrays, false when a per-row block writes it: an
// input, boxed, dependent or define field, or a value columnFor refuses.
func (g *gen) fieldColumn(row string, f *ir.Field, rows []*value.Record) ([]string, bool) {
	if f.Input != nil || f.Optional && f.Type.Kind == types.Never || g.boxed[f] || ir.HeldApp(f.Type) != nil {
		return nil, false
	}
	if _, _, define := g.pl.DefineValue(f); define {
		return nil, false
	}
	m, err := g.member(f.Name)
	if err != nil {
		return nil, false
	}
	vals := make([]value.Value, len(rows))
	for i, r := range rows {
		vals[i] = g.fieldOf(r, f.Name)
	}
	return g.columnFor(place{key: row + memberAccess + m, ref: row + memberAccess + g.pl.RefMember(f.Name)}, f.Type, f.Optional, vals)
}

// storedColumn is a precomputed method's result for every row from arrays, false for a lookup.
func (g *gen) storedColumn(row string, fn *ir.ExportFn, rows []*value.Record) ([]string, bool) {
	if fn.Kind != ir.FnPrecomputed {
		return nil, false
	}
	m, err := g.member(fn.Name)
	if err != nil {
		return nil, false
	}
	vals := make([]value.Value, len(rows))
	for i, r := range rows {
		vals[i] = g.instanceOf(fn, r).Result
	}
	t, optional := resultType(fn.Result)
	return g.columnFor(place{key: row + memberAccess + m, ref: row + memberAccess + g.pl.RefMember(fn.Name)}, t, optional, vals)
}

// columnFor is the statements filling slot p of every row from arrays of vals, its resolved
// entry too; false for a value arrays cannot hold, a resolved optional ref or list of refs, or a
// value fillSlot leaves as constructed where the column would not.
func (g *gen) columnFor(p place, t ir.TypeRef, optional bool, vals []value.Value) ([]string, bool) {
	list, isRef := refSlot(t)
	resolves := isRef && g.pl.Resolves(t, nil)
	for _, v := range vals {
		if v == nil || absent(v) && !optional {
			return nil, false
		}
	}
	switch {
	case resolves && (list || optional):
		return nil, false
	case t.Kind == types.List:
		return g.listColumn(p.key, t, optional, vals)
	}
	cell := t
	if optional {
		cell = ir.TypeRef{Kind: types.Optional, Elem: &t}
	}
	name, ok := g.columnArray(cell, vals)
	if !ok {
		return nil, false
	}
	out := []string{fmt.Sprintf(assignFormat, p.key, columnAt(name))}
	if resolves {
		pos, ok := g.entryPositions(t, vals)
		if !ok {
			return nil, false
		}
		out = append(out, fmt.Sprintf(assignFormat, p.ref, fmt.Sprintf(entryAtFormat, dataPrefix+g.pl.DataMember(t.Ref.Value), columnAt(pos))))
	}
	return out, true
}

// columnArray writes vals, each of type t, as one array; false for a type arrayElem refuses.
func (g *gen) columnArray(t ir.TypeRef, vals []value.Value) (string, bool) {
	elem, ok := g.arrayElem(t)
	if !ok {
		return "", false
	}
	items := make([]string, len(vals))
	for i, v := range vals {
		items[i] = g.arrayText(func() string { return g.element(t, v) })
	}
	return g.pool.array(elem, items), true
}

// entryPositions writes the positions of the entries refs vals name, as one array.
func (g *gen) entryPositions(t ir.TypeRef, vals []value.Value) (string, bool) {
	at := make([]string, len(vals))
	for i, v := range vals {
		r, isRef := v.(*value.Ref)
		if !isRef {
			return "", false
		}
		pos, ok := g.bk.index[t.Ref.Value][r.Key]
		if !ok {
			return "", false
		}
		at[i] = strconv.Itoa(pos)
	}
	return g.pool.array(cppSize, at), true
}

// listColumn is a list member of every row as a slice of one flat array, the rows' bounds in a
// second; nothing when every list is empty, as constructed. An optional list keeps per-row blocks.
func (g *gen) listColumn(lhs string, t ir.TypeRef, optional bool, vals []value.Value) ([]string, bool) {
	if optional || t.KeyedBy != nil || t.Elem == nil {
		return nil, false
	}
	elem, ok := g.arrayElem(*t.Elem)
	if !ok {
		return nil, false
	}
	var items []string
	bounds := []string{strconv.Itoa(0)}
	for _, v := range vals {
		l, isList := v.(*value.List)
		if !isList {
			return nil, false
		}
		for _, e := range l.Elems {
			items = append(items, g.arrayText(func() string { return g.element(*t.Elem, e) }))
		}
		bounds = append(bounds, strconv.Itoa(len(items)))
	}
	if len(items) == 0 {
		return nil, true
	}
	flat, at := g.pool.array(elem, items), g.pool.array(cppSize, bounds)
	return []string{fmt.Sprintf(assignFormat, lhs, fmt.Sprintf(sliceFormat, g.storage(t), flat, at, flat, at))}, true
}
