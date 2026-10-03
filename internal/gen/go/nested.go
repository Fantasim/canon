package gogen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// nestedRecord is the record of this package a table field holds: stage E refuses any other, and a baked one whose id type is a table value's enum (E8019 TableField, CODEGEN.md §4.2, §5.3).
func (g *gen) nestedRecord(t ir.TypeRef) *ir.Record {
	rec, ok := g.sub(t.Elem).Named.(*ir.Record)
	if !ok || rec.Pkg != g.p.Name || !g.isData() && g.tableOf[rec] != nil {
		g.fail(newDetail(ErrMalformed, g.at, tableFieldFormat, g.at)) // E8019 TableField
		return &ir.Record{}
	}
	return rec
}

// tableType is a table field's getter type, rt.KeyedList[<id type>, T] (CODEGEN.md §4.2).
func (g *gen) tableType(t ir.TypeRef) string {
	rec := g.nestedRecord(t)
	return g.rt() + keyedListType + lbracket + g.idType(rec) + listSep + g.goName(rec) + rbracket
}

// readNested reads a nested table: an object keyed by entry id, each row a record read with its key as its id and its `$retired` (WIRE.md §5.7).
func (g *gen) readNested(b *strings.Builder, t ir.TypeRef, raw string, loc location) string {
	rec, lc := g.nestedRecord(t), g.lc
	ids, rows, retired := g.temp(tempKey), g.temp(tempRaw), g.temp(tempOK)
	values, keys, i, x := g.temp(tempValue), g.temp(tempKey), g.temp(tempIndex), g.temp(tempElem)
	fmt.Fprintf(b, nestedOpenFormat, ids, rows, retired, lc.Err, helperTable, lc.Name, g.locExpr(loc.dot()), raw)
	fmt.Fprintf(b, nestedRowsFormat, values, g.goName(rec), rows, keys, g.idType(rec), i, x, lc.Err, g.decodeFunc(rec), lc.Name,
		g.locExpr(loc.dot().arg(ids+lbracket+i+rbracket).dot()), ir.GoIDStore, ir.GoRetiredStore, ids, retired)
	return g.rt() + makeKeyedList + values + listSep + keys + rparen
}

// tableExpr is a table field's value: its rows, each with its id, and their keys (CODEGEN.md §4.2).
func (g *gen) tableExpr(t ir.TypeRef, v *value.Table) string {
	rec := g.nestedRecord(t)
	name, id := g.goName(rec), g.idType(rec)
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

// rowLit is a nested table's entry R{…}, its id first, and its key.
func (g *gen) rowLit(rec *ir.Record, r *value.Record) (lit, key string) {
	if r.Ident == nil {
		g.failf(ErrMalformed, tableEntryFormat, g.at)
		return nilLit, nilLit
	}
	key = strconv.Quote(r.Ident.Key.S)
	parts := []pair{{ir.GoIDStore, key}}
	if r.Ident.Retired {
		parts = append(parts, pair{ir.GoRetiredStore, trueLit})
	}
	return compositeLit(g.typeName(rec), append(parts, g.bodyLit(g.bodyOf(rec), r)...)), key
}

// nestedIDs writes the string id type of each record only a table field holds (CODEGEN.md §5.3: a nested table has no id constants).
func (g *gen) nestedIDs() {
	for _, t := range g.p.Types {
		if rec, ok := t.(*ir.Record); ok && g.names.NestedRow(rec) && g.tableOf[rec] == nil {
			g.printf(stringTypeFormat, g.idType(rec))
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
