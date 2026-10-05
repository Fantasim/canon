package cppgen

import (
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// nestedRecord is the record of this package a table field holds: stage E refuses any other (E8019 TableField, CODEGEN.md §4.2, §5.3).
func (g *gen) nestedRecord(t ir.TypeRef) *ir.Record {
	if t.Elem != nil {
		if rec, ok := t.Elem.Named.(*ir.Record); ok && rec.Pkg == g.p.Name {
			return rec
		}
	}
	g.malformed(tableFieldText, g.at) // E8019 TableField
	return &ir.Record{}
}

// tableStorage is a table field's storage, a canon::KeyedList of its rows by id, a string in every mode this generator writes (CODEGEN.md §4.2, §5.3).
func (g *gen) tableStorage(t ir.TypeRef) string {
	return fmt.Sprintf(keyedListFormat, cppString, g.typeName(g.nestedRecord(t)))
}

// nestedRows marks the records a table field holds as entries (an id and a retired flag) and befriends the classes decoding them, which set both (CODEGEN.md §4.2, §5.3).
func (g *gen) nestedRows() {
	for _, c := range g.declared() {
		fields, _ := c.shape()
		for _, f := range fields {
			walkTypeRef(f.Type, func(t ir.TypeRef) { g.markRow(c, t) })
		}
	}
}

// markRow marks the row record of t when it is a table field of class c.
func (g *gen) markRow(c class, t ir.TypeRef) {
	if t.Kind != types.Table || t.Elem == nil {
		return
	}
	rec, ok := t.Elem.Named.(*ir.Record)
	if !ok || !g.pl.NestedRow(rec) {
		return
	}
	g.entries[rec] = true
	if friend := g.className(c); friend != g.typeName(rec) && !slices.Contains(g.pairsFriends[rec], friend) {
		g.pairsFriends[rec] = append(g.pairsFriends[rec], friend)
	}
}

// walkTypeRef calls visit on t and the types it is made of.
func walkTypeRef(t ir.TypeRef, visit func(ir.TypeRef)) {
	visit(t)
	for _, part := range []*ir.TypeRef{t.Elem, t.Key} {
		if part != nil {
			walkTypeRef(*part, visit)
		}
	}
}

// readsNested reports a package one of whose loaders can read a nested table, in a class of its own, in the type of one of its values, or in a package it imports: or a map, its loaders then keep each object's file order (WIRE.md §5.7, §5.8).
func (g *gen) readsNested() bool {
	seen := map[any]bool{}
	var holds func(t ir.TypeRef) bool
	var fieldsHold func(fields []*ir.Field) bool
	holds = func(t ir.TypeRef) bool {
		found := false
		walkTypeRef(t, func(x ir.TypeRef) {
			switch {
			case x.Kind == types.Table, x.Kind == types.Map:
				found = true
			case x.Named == nil || seen[x.Named]:
			default:
				seen[x.Named] = true
				found = found || namedHolds(x.Named, fieldsHold)
			}
		})
		return found
	}
	fieldsHold = func(fields []*ir.Field) bool {
		return slices.ContainsFunc(fields, func(f *ir.Field) bool { return holds(f.Type) })
	}
	for _, c := range g.declared() {
		fields, _ := c.shape()
		if fieldsHold(fields) {
			return true
		}
	}
	for _, v := range g.values {
		root := v.Type
		if root.Kind == types.Table && root.Elem != nil {
			root = *root.Elem // the value's own table is a loader's, not a nested one
		}
		if holds(root) {
			return true
		}
	}
	return false
}

// namedHolds applies fieldsHold to the fields of a record, or of every case of a variant.
func namedHolds(named ir.Type, fieldsHold func([]*ir.Field) bool) bool {
	switch x := named.(type) {
	case *ir.Record:
		return fieldsHold(x.Fields)
	case *ir.Variant:
		return slices.ContainsFunc(x.Cases, func(c *ir.Case) bool { return fieldsHold(c.Fields) })
	}
	return false
}

// decodeNested reads a nested table: an object keyed by entry id, each row decoded as its record, with its key as its id and its `$retired` its retired flag (WIRE.md §5.7).
func (g *gen) decodeNested(depth int, src, key string, l leaf) {
	rec := g.nestedRecord(l.t)
	elem, d := g.typeName(rec), depth
	g.c.linef(depth, pushFormat, key)
	g.c.linef(depth, tableEntriesFormat, d)
	g.c.linef(depth, tableReadFormat, src, d)
	g.c.linef(depth+1, tableRowsFormat, elem, d)
	g.c.linef(depth+1, tableKeysLine, d)
	g.c.linef(depth+1, tableLoopFormat, d)
	g.c.linef(depth+depthTwo, tableRowPush, d)
	g.c.linef(depth+depthTwo, tableDecodeFormat, g.decodeFunc(*l.t.Elem), d)
	g.c.linef(depth+depthTwo, popLine)
	g.c.linef(depth+depthTwo, tableIDFormat, d)
	g.c.linef(depth+depthTwo, tableRetiredFormat, d)
	g.c.linef(depth+depthTwo, tableKeyPushFormat, d)
	g.c.linef(depth+1, closeBrace)
	g.c.linef(depth+1, tableFromFormat, l.dst, elem, d)
	g.c.linef(depth, closeBrace)
	g.c.linef(depth, popLine)
}

// walkTable resolves the refs of a table field's rows, each on the decoder's path by its id.
func (g *gen) walkTable(depth int, t ir.TypeRef, expr, key string) {
	n := fmt.Sprintf(indexVarFormat, depth)
	elem := fmt.Sprintf(keyedElemFormat, g.storage(*t.Elem), expr, n)
	g.c.linef(depth, pushFormat, key)
	g.c.linef(depth, lenLoopFormat, n, n, expr, n)
	g.walkValue(depth+1, *t.Elem, false, elem, fmt.Sprintf(tableIDExprFormat, expr, n))
	g.c.linef(depth, closeBrace)
	g.c.linef(depth, popLine)
}

// tableLit is the literal of a table field's empty default; a row needs a record literal, which E8019 RecordConstant refuses (CODEGEN.md §5.1).
func (g *gen) tableLit(t ir.TypeRef, v *value.Table) string {
	if len(v.Entries) > 0 {
		g.malformed(fmt.Sprintf(valueFormat, v), g.at) // E8019 RecordConstant
	}
	return g.storage(t) + openBrace + closeBrace
}
