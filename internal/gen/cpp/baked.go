package cppgen

import (
	_ "embed"
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

//go:embed text/baked_access.txt
var bakedAccessText string

// bakedIndex is what a baked emit's Build reads: each container's entry positions by key, each stored method's results by receiver (CODEGEN.md §5.9, §5.10).
type bakedIndex struct {
	index     map[string]map[value.Key]int
	instances map[*ir.ExportFn]map[*value.Record]*ir.Instance
}

// indexBaked indexes the emitted containers and the stored methods' instances, and refuses a @reload value (E8202).
func (g *gen) indexBaked() {
	g.bk = &bakedIndex{index: map[string]map[value.Key]int{}, instances: map[*ir.ExportFn]map[*value.Record]*ir.Instance{}}
	for _, v := range g.values {
		if v.Reload {
			g.malformed(bakedReload, v.Name) // E8202
		}
		if ir.IsContainer(v) {
			g.bk.index[v.Name] = g.entryIndex(v)
		}
	}
	for _, c := range g.declared() {
		_, fns := c.shape()
		for _, fn := range fns {
			g.indexInstances(fn)
		}
	}
}

func (g *gen) indexInstances(fn *ir.ExportFn) {
	byRecv := map[*value.Record]*ir.Instance{}
	for _, in := range fn.Instances {
		byRecv[in.Recv] = in
	}
	g.bk.instances[fn] = byRecv
}

// rows are the entries of a table or keyed-list value, in entry order.
func (g *gen) rows(v *ir.Value) []*value.Record {
	switch x := v.V.(type) {
	case *value.Table:
		return x.Entries
	case *value.List:
		out := make([]*value.Record, 0, len(x.Elems))
		for _, e := range x.Elems {
			if r, ok := e.(*value.Record); ok {
				out = append(out, r)
			}
		}
		if len(out) == len(x.Elems) {
			return out
		}
	}
	g.malformed(fmt.Sprintf(valueFormat, v.V), v.Name)
	return nil
}

// entryIndex maps each entry's key to its position; a table's rows follow its ids, which number its id enum (CODEGEN.md §5.3).
func (g *gen) entryIndex(v *ir.Value) map[value.Key]int {
	index := map[value.Key]int{}
	table := v.Type.Kind == types.Table
	for i, r := range g.rows(v) {
		if r.Ident == nil || table && (i >= len(v.IDs) || v.IDs[i] != r.Ident.Key.S) {
			g.malformed(bakedEntryKey, v.Name)
			continue
		}
		index[r.Ident.Key] = i
	}
	if table && len(index) != len(v.IDs) {
		g.malformed(bakedEntryKey, v.Name)
	}
	return index
}

// instanceOf is fn's precomputed result for the receiver r.
func (g *gen) instanceOf(fn *ir.ExportFn, r *value.Record) *ir.Instance {
	if _, indexed := g.bk.instances[fn]; !indexed { // a method of another package's class (CODEGEN.md §5.14)
		g.indexInstances(fn)
	}
	if in := g.bk.instances[fn][r]; in != nil {
		return in
	}
	g.malformed(bakedNoInstance, g.at+qnameSep+fn.Name)
	return &ir.Instance{}
}

// bakedAccess is detail::<P>Access: Data holds every value and every package fn's cells that are not constexpr, built once by Build in a function-local static (CODEGEN.md §5.9, §7.3).
func (g *gen) bakedAccess() {
	var members, body writer
	for _, v := range g.values {
		leave := g.enter(v.Name)
		g.dataMembers(&members, v)
		leave()
	}
	for _, fn := range g.p.Fns {
		if fn.Kind != ir.FnTranslated && !g.pl.Constexpr(fn) {
			g.fnCellMembers(&members, fn)
		}
	}
	if members.String() == "" {
		return
	}
	g.pool = bakedBulk()
	defer func() { g.pool = nil }()
	for _, v := range g.values {
		if ir.IsContainer(v) {
			leave := g.enter(v.Name)
			g.allocate(&body, v)
			leave()
		}
	}
	for _, v := range g.values {
		leave := g.enter(v.Name)
		g.fillData(&body, v)
		leave()
	}
	for _, fn := range g.p.Fns {
		if fn.Kind != ir.FnTranslated && !g.pl.Constexpr(fn) {
			leave := g.enter(fn.Name)
			g.fillFnCells(&body, fn)
			leave()
		}
	}
	if g.pool.n > 0 {
		g.c.line(anonOpen)
		g.c.blank()
		g.c.write(g.pool.w.String())
		g.c.line(anonClose)
		g.c.blank()
	}
	access := g.pl.AccessName()
	decls, calls, defs := buildParts(access, body.String())
	g.c.printf(bakedAccessText, access, members.String(), decls, calls, defs)
}

// dataMembers are a value's members of Data: its container, record or storage, and a resolved ref's entries.
func (g *gen) dataMembers(w *writer, v *ir.Value) {
	m := g.pl.DataMember(v.Name)
	if ir.IsContainer(v) {
		w.linef(depthTwo, memberFormat, g.pl.ContainerName(v), m, "")
		return
	}
	t, optional := resultType(v.Type)
	w.linef(depthTwo, memberFormat, g.memberType(t, optional), m, g.memberInit(t, optional))
	if list, isRef := refSlot(t); isRef && g.pl.Resolves(t, nil) {
		storage, init := g.refStorageOf(t, list, optional)
		w.linef(depthTwo, memberFormat, storage, g.pl.RefData(v.Name), init)
	}
}

// refStorageOf is how a resolved ref's entries are held: a pointer, a vector of them, optional for an optional list (CODEGEN.md §5.8).
func (g *gen) refStorageOf(t ir.TypeRef, list, optional bool) (storage, init string) {
	target := t
	if list {
		target = *t.Elem
	}
	ptr := fmt.Sprintf(constPtrFormat, g.valueElem(g.valueNamed(target.Ref.Value)))
	switch {
	case list && optional:
		return fmt.Sprintf(optionalFormat, fmt.Sprintf(vectorFormat, ptr)), ""
	case list:
		return fmt.Sprintf(vectorFormat, ptr), ""
	}
	return ptr, initNull
}

// allocate gives a container its rows first, so that any entry can point at any other: the rows are sized once, with no key and no index, as its Find and FindBy<F> search constant tables (CODEGEN.md §5.9, DECISIONS 432).
func (g *gen) allocate(w *writer, v *ir.Value) {
	w.linef(1, resizeFormat, dataPrefix+g.pl.DataMember(v.Name)+memberAccess+rowsField, len(g.rows(v)))
}

// keyFieldOf is a keyed list's key field, nil for a table, whose keys are its ids.
func (g *gen) keyFieldOf(v *ir.Value) *ir.Field {
	if v.Type.KeyedBy == nil {
		return nil
	}
	return g.keyField(v.Type)
}

// fieldOf is the value of field name of r; a record without it is malformed IR.
func (g *gen) fieldOf(r *value.Record, name string) value.Value {
	v, ok := ir.FieldValue(r, name)
	if !ok {
		g.malformed(bakedNoField, g.at+qnameSep+name)
	}
	return v
}

// fillData writes a value into Data: each container's rows, or the value as a field of its type holds it.
func (g *gen) fillData(w *writer, v *ir.Value) {
	data := dataPrefix + g.pl.DataMember(v.Name)
	if !ir.IsContainer(v) {
		t, optional := resultType(v.Type)
		g.fillSlot(at{w, 1}, place{key: data, ref: dataPrefix + g.pl.RefData(v.Name)}, t, optional, v.V)
		return
	}
	rec, ok := v.Type.Elem.Named.(*ir.Record)
	if !ok {
		g.malformed(fmt.Sprintf(typeFormat, v.Type.Elem.Named), v.Name)
		return
	}
	rows := g.rows(v)
	done := g.fillColumns(w, v, rec, rows)
	for i, r := range rows {
		var block writer
		block.linef(1, openBrace)
		block.linef(depthTwo, rowRefFormat, rowVar(depthTwo), data, i)
		head := block.size()
		if rec.Pkg != g.p.Name {
			g.fillRow(at{&block, depthTwo}, rowVar(depthTwo), rec, r, g.rowIDLit(rec, r))
		} else {
			g.fillRecordExcept(at{&block, depthTwo}, rowVar(depthTwo), class{rec: rec}, r, done)
		}
		if block.size() > head || done == nil { // a row the columns wrote whole needs no block
			block.linef(1, closeBrace)
			w.write(block.String())
		}
	}
}

// rowVar is the local naming the record a block at depth fills.
func rowVar(depth int) string { return fmt.Sprintf(rowVarFormat, depth) }
