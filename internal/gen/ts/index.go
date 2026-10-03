package tsgen

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// indexEntries takes stage E's rows and loose rows (ir.TSRows, CODEGEN.md §5.3, §5.4), and counts the table types holding each row: its id is its table's id type only when one does.
func (g *gen) indexEntries() {
	g.entries, g.loose = ir.TSRows(g.p, g.e)
	count := func(t ir.TypeRef) {
		if r, ok := rowRecord(t); ok {
			g.rowSites[r]++
		}
	}
	for _, v := range g.emitted {
		walkType(v.Type, count)
	}
	for _, t := range g.p.Types {
		for _, f := range fieldsOf(t) {
			walkType(f.Type, count)
		}
	}
}

// rowRecord is the record a table type holds.
func rowRecord(t ir.TypeRef) (*ir.Record, bool) {
	if t.Kind != types.Table || t.Elem == nil {
		return nil, false
	}
	r, ok := t.Elem.Named.(*ir.Record)
	return r, ok
}

// walkType calls visit on t and every type it holds.
func walkType(t ir.TypeRef, visit func(ir.TypeRef)) {
	visit(t)
	if t.Elem != nil {
		walkType(*t.Elem, visit)
	}
	if t.Key != nil {
		walkType(*t.Key, visit)
	}
}

// fieldsOf are the fields of a record, or of every case of a variant.
func fieldsOf(t ir.Type) []*ir.Field {
	switch x := t.(type) {
	case *ir.Record:
		return x.Fields
	case *ir.Variant:
		var out []*ir.Field
		for _, c := range x.Cases {
			out = append(out, c.Fields...)
		}
		return out
	}
	return nil
}

// instanceOf is the precomputed result of fn for the receiver recv; each fn's receivers are indexed once.
func (g *gen) instanceOf(fn *ir.ExportFn, recv *value.Record) *ir.Instance {
	byRecv := g.instances[fn]
	if byRecv == nil {
		byRecv = map[*value.Record]*ir.Instance{}
		for _, in := range fn.Instances {
			byRecv[in.Recv] = in
		}
		g.instances[fn] = byRecv
	}
	if in := byRecv[recv]; in != nil {
		return in
	}
	g.failf(ErrMalformed, malformedNoInstance, fn.Name, g.at)
	return &ir.Instance{}
}
