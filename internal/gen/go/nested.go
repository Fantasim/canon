package gogen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// nestedRecord is the record a table field holds: stage E refuses any other, and a baked own one whose id type is a table value's enum (E8019 TableField, CODEGEN.md §4.2, §5.3).
func (g *gen) nestedRecord(t ir.TypeRef) *ir.Record {
	rec, ok := g.sub(t.Elem).Named.(*ir.Record)
	if !ok || rec.Pkg == g.p.Name && g.holder() == g.p.Name && !g.isData() && g.tableOf[rec] != nil {
		g.fail(newDetail(ErrMalformed, g.at, tableFieldFormat, g.at)) // E8019 TableField
		return &ir.Record{}
	}
	return rec
}

// nestedID is the id type of a table field of rec in the holding package (CODEGEN.md §4.2, §5.3).
func (g *gen) nestedID(rec *ir.Record) string {
	return g.qualify(g.holder(), g.idType(rec))
}

// tableType is a table field's getter type, rt.KeyedList[<id type>, T], T the holder's row type of another package's record (CODEGEN.md §4.2).
func (g *gen) tableType(t ir.TypeRef) string {
	rec := g.nestedRecord(t)
	return g.rt() + keyedListType + lbracket + g.nestedID(rec) + listSep + g.rowElem(rec, true) + rbracket
}

// readNested reads a nested table: an object keyed by entry id, each row a record read with its key as its id and its `$retired` (WIRE.md §5.7); another package's record goes into the holder's row.
func (g *gen) readNested(b *strings.Builder, t ir.TypeRef, raw string, loc location) string {
	rec, lc := g.nestedRecord(t), g.lc
	ids, rows, retired := g.temp(tempKey), g.temp(tempRaw), g.temp(tempOK)
	values, keys, i, x := g.temp(tempValue), g.temp(tempKey), g.temp(tempIndex), g.temp(tempElem)
	elem, id := g.rowElem(rec, true), g.nestedID(rec)
	g.called[helperTable] = true
	fmt.Fprintf(b, nestedOpenFormat, ids, rows, retired, lc.Err, helperTable, lc.Name, g.locExpr(loc.dot()), raw)
	fmt.Fprintf(b, nestedRowsFormat, values, elem, rows, keys, id, i, x)
	row, at := values+lbracket+i+rbracket, g.locExpr(loc.dot().arg(ids+lbracket+i+rbracket).dot())
	idExpr, retiredExpr := id+lparen+ids+lbracket+i+rbracket+rparen, retired+lbracket+i+rbracket
	switch h := g.holder(); {
	case rec.Pkg == h && h == g.p.Name:
		fmt.Fprintf(b, keysFormat, lc.Err, g.decodeFunc(rec), lc.Name, at, x, ampersand+row)
		fmt.Fprintf(b, setRowFormat, row, ir.GoIDStore, ir.GoRetiredStore, idExpr, retiredExpr)
	case h == g.p.Name:
		fmt.Fprintf(b, keysFormat, lc.Err, g.decodeFunc(rec), lc.Name, at, x, ampersand+row+dot+rowRecordStore)
		fmt.Fprintf(b, setRowFormat, row, ir.GoIDStore, ir.GoRetiredStore, idExpr, retiredExpr)
	default:
		r := g.temp(tempElem)
		fmt.Fprintf(b, newValueFormat, r, g.typeName(rec))
		fmt.Fprintf(b, keysFormat, lc.Err, g.decodeFunc(rec), lc.Name, at, x, r)
		fmt.Fprintf(b, cellStoreFormat, row, g.heldRowHook(rec)+callArgs([]string{pointer + r, idExpr, retiredExpr}))
	}
	fmt.Fprintf(b, cellStoreFormat, keys+lbracket+i+rbracket, row+dot+g.rowID())
	b.WriteString(closeBrace)
	return g.rt() + makeKeyedList + values + listSep + keys + rparen
}

// rowID reads a row's id: its storage in this package, its ID() in another.
func (g *gen) rowID() string {
	if g.holder() != g.p.Name {
		return ir.GoID + callSuffix
	}
	return ir.GoIDStore
}

// heldRowHook is the holder's hook that makes a row of its table field of rec: its row type's for another package's record, its entry hook for its own (CODEGEN.md §5.14; log-2026-10-06 "U1 review" 1).
func (g *gen) heldRowHook(rec *ir.Record) string {
	h := g.holder()
	if rec.Pkg != h {
		return g.qualify(h, g.names.RowHook(rec))
	}
	return g.qualify(h, g.names.EntryHook(rec))
}

// tableExpr is a table field's value: its rows, each with its id, and their keys (CODEGEN.md §4.2).
func (g *gen) tableExpr(t ir.TypeRef, v *value.Table) string {
	rec := g.nestedRecord(t)
	name, id := g.rowElem(rec, true), g.nestedID(rec)
	if len(v.Entries) == 0 {
		return g.rt() + keyedListType + lbracket + id + listSep + name + rbracket + emptyBraces
	}
	rows := make([]string, len(v.Entries))
	keys := make([]string, len(v.Entries))
	for i, r := range v.Entries {
		rows[i], keys[i] = g.rowLit(rec, r)
	}
	return g.rt() + makeKeyedList + sliceOf + name + braced(elide(name, rows)) + listSep + sliceOf + id + braced(keys) + rparen
}

// rowLit is a nested table's entry R{…}, its id first, and its key; another package's record, or the holder's own built elsewhere, goes through a hook.
func (g *gen) rowLit(rec *ir.Record, r *value.Record) (lit, key string) {
	if r.Ident == nil {
		g.failf(ErrMalformed, tableEntryFormat, g.at)
		return nilLit, nilLit
	}
	key = strconv.Quote(r.Ident.Key.S)
	switch h := g.holder(); {
	case rec.Pkg != h:
		return g.foreignRowLit(rec, r, key), key
	case h != g.p.Name:
		retired := strconv.FormatBool(r.Ident.Retired)
		return g.heldRowHook(rec) + callArgs([]string{g.recordLit(rec, r), key, retired}), key
	}
	parts := []pair{{ir.GoIDStore, key}}
	if r.Ident.Retired {
		parts = append(parts, pair{ir.GoRetiredStore, trueLit})
	}
	return compositeLit(g.typeName(rec), append(parts, g.bodyLit(g.bodyOf(rec), r)...)), key
}

// nestedIDs writes the string id type of each record only a table field holds, of this package or another (CODEGEN.md §5.3: a nested table has no id constants).
func (g *gen) nestedIDs() {
	for _, t := range g.p.Types {
		if rec, ok := t.(*ir.Record); ok && g.names.NestedRow(rec) && g.tableOf[rec] == nil {
			g.printf(stringTypeFormat, g.idType(rec))
		}
	}
	for _, row := range g.names.Rows() {
		if row.Field != nil && g.tableOf[row.Record] == nil {
			g.printf(stringTypeFormat, g.idType(row.Record))
		}
	}
}

// walkTable resolves the refs of a table field's rows, each named by its id.
func (g *gen) walkTable(b *strings.Builder, t ir.TypeRef, expr string, loc location) {
	rec, lc := g.nestedRecord(t), g.lc
	i := g.temp(tempIndex)
	fmt.Fprintf(b, walkListFormat, i, expr)
	row := expr + atCall + i + rparen
	prefix := g.locExpr(loc.dot().arg(row + dot + ir.GoIDStore).dot())
	fmt.Fprintf(b, walkOneFormat, row, lc.Err, g.resolveFunc(rec), lc.Name, prefix, lc.Ctx)
	b.WriteString(closeBrace)
}
