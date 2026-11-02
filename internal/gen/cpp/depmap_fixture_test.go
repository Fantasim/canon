package cppgen_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const dmPkg = "demo.depmap"

// dmWide is the id count of the table `wide`: past gen/cpp's array threshold, so a dependent map over it is read from arrays.
const dmWide = 9

// dmGrid is the length of the field `grid`: past gen/cpp's array threshold.
const dmGrid = 5

// depmap is the IR of the dependent-map fixture, shared by its builders.
type depmap struct {
	row, cell, pt, holder     *ir.Record
	shape                     *ir.Variant
	rowT, cellT, ptT, holderT *types.RecordType
	weightsCase, emptyCase    *types.CaseType
	byRows, byWide, byRecs    ir.TypeRef
	byNest                    ir.TypeRef
	kind                      types.Kind
}

func dmRecordType(name string, fields ...string) *types.RecordType {
	rt := &types.RecordType{Pkg: dmPkg, Name: name}
	for i, f := range fields {
		rt.Fields = append(rt.Fields, &types.Field{Name: f, Index: i})
	}
	return rt
}

// dmRef is a ref into table of this package, as stage E types a dependent map's key (`{k in table: T}`).
func dmRef(table string, elem ir.Type) ir.TypeRef {
	key := tString
	return ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: dmPkg, Value: table, Elem: elem}}
}

// dmMap is `{k in table: v}` of kind types.DepMap (CODEGEN.md §4.2), or its plain-map twin of kind types.Map, written the same.
func dmMap(kind types.Kind, table string, elem ir.Type, v ir.TypeRef) ir.TypeRef {
	k := dmRef(table, elem)
	return ir.TypeRef{Kind: kind, Key: &k, Elem: &v}
}

func dmEntries(keys []string, vals ...value.Value) *value.Map {
	m := &value.Map{}
	for i, k := range keys {
		m.Keys, m.Vals = append(m.Keys, key(k)), append(m.Vals, vals[i])
	}
	return m
}

func dmWideIDs() []string {
	ids := make([]string, dmWide)
	for i := range ids {
		ids[i] = fmt.Sprintf("w%d", i)
	}
	return ids
}

func (d *depmap) types() {
	d.row = &ir.Record{Pkg: dmPkg, Name: "Row", Doc: "A row.", Fields: []*ir.Field{field("n", "n", "Its number.", tInt)}}
	d.rowT = dmRecordType("Row", "n")
	d.byRows = dmMap(d.kind, "rows", d.row, tInt)
	d.cell = &ir.Record{Pkg: dmPkg, Name: "Cell", Doc: "A wide row.", Fields: []*ir.Field{field("n", "n", "Its number.", tInt), field("links", "links", "Its links.", d.byRows)}}
	d.cellT = dmRecordType("Cell", "n", "links")
	d.byWide = dmMap(d.kind, "wide", d.cell, tInt)
	d.byNest = dmMap(d.kind, "wide", d.cell, listOf(tInt))
	d.pt = &ir.Record{Pkg: dmPkg, Name: "Pt", Doc: "A point.", Fields: []*ir.Field{field("n", "n", "Its number.", tInt)}}
	d.ptT = dmRecordType("Pt", "n")
	d.byRecs = dmMap(d.kind, "rows", d.row, ir.TypeRef{Kind: types.Record, Named: d.pt})
	d.shape = &ir.Variant{Pkg: dmPkg, Name: "Shape", Doc: "A shape.", Tag: "kind", Cases: []*ir.Case{
		{Name: "weights", Wire: "weights", Fields: []*ir.Field{field("w", "w", "Its weights.", d.byRows)}},
		{Name: "empty", Wire: "empty"},
	}}
	d.weightsCase = &types.CaseType{Name: "weights", Index: 0, Fields: []*types.Field{{Name: "w"}}}
	d.emptyCase = &types.CaseType{Name: "empty", Index: 1}
	opt := field("opt", "opt", "Weights, if any.", d.byRows)
	opt.Optional = true
	absent := field("absent", "absent", "Weights, if none.", d.byRows)
	absent.Optional = true
	d.holder = &ir.Record{Pkg: dmPkg, Name: "Holder", Doc: "A holder.", Fields: []*ir.Field{
		field("m", "m", "The weights by row.", d.byRows),
		field("many", "many", "A list of them.", listOf(d.byRows)),
		opt, absent,
		field("byName", "byName", "Maps of them.", ir.TypeRef{Kind: types.Map, Key: &tString, Elem: &d.byRows}),
		field("recs", "recs", "Records by row.", d.byRecs),
		field("shape", "shape", "A case holding one.", ir.TypeRef{Kind: types.Variant, Named: d.shape}),
		field("big", "big", "Past the array threshold.", d.byWide),
		field("grid", "grid", "A short list of small ones: the dependent levels decide it is read from arrays.", listOf(d.byRows)),
		field("nest", "nest", "Lists by wide row.", d.byNest),
	}}
	d.holderT = dmRecordType("Holder", "m", "many", "opt", "absent", "byName", "recs", "shape", "big", "grid", "nest")
	d.cell.Methods = []*ir.ExportFn{{Name: "hops", Kind: ir.FnPrecomputed, Result: d.byRows, Doc: "Its hops by row."}}
	d.row.Methods = []*ir.ExportFn{{Name: "scores", Kind: ir.FnPrecomputed, Result: d.byRows, Doc: "Its scores by row."}}
}

func (d *depmap) rows() []*ir.Value {
	coll := func(table string) *types.Collection {
		return &types.Collection{Kind: types.CollLet, Pkg: dmPkg, Name: table}
	}
	entry := func(t *types.RecordType, table, id string, fields ...value.Value) *value.Record {
		return &value.Record{T: t, Fields: fields, Ident: &value.Identity{Coll: coll(table), Key: value.Key{S: id}}}
	}
	rows := []*value.Record{entry(d.rowT, "rows", "a", num(1)), entry(d.rowT, "rows", "b", num(2))}
	scores := d.row.Methods[0]
	for i, r := range rows {
		scores.Instances = append(scores.Instances, &ir.Instance{Recv: r, Result: dmEntries([]string{"a", "b"}, num(int64(10*(i+1))), num(int64(10*(i+1)+1)))})
	}
	var wide []*value.Record
	hops := d.cell.Methods[0]
	for i, id := range dmWideIDs() {
		wide = append(wide, entry(d.cellT, "wide", id, num(int64(i)), dmEntries([]string{"a", "b"}, num(int64(i)), num(int64(2*i)))))
		hops.Instances = append(hops.Instances, &ir.Instance{Recv: wide[i], Result: dmEntries([]string{"b"}, num(int64(3*i)))})
	}
	rowElem, cellElem := ir.TypeRef{Kind: types.Record, Named: d.row}, ir.TypeRef{Kind: types.Record, Named: d.cell}
	return []*ir.Value{
		{Name: "rows", Doc: "The rows.", IDs: []string{"a", "b"}, Type: ir.TypeRef{Kind: types.Table, Elem: &rowElem}, V: &value.Table{Entries: rows}},
		{Name: "wide", Doc: "The wide rows.", IDs: dmWideIDs(), Type: ir.TypeRef{Kind: types.Table, Elem: &cellElem}, V: &value.Table{Entries: wide}},
	}
}

func (d *depmap) holderValue() *value.Record {
	rec := func(n int64) value.Value { return &value.Record{T: d.ptT, Fields: []value.Value{num(n)}} }
	ab := []string{"a", "b"}
	wideVals := make([]value.Value, dmWide)
	for i := range wideVals {
		wideVals[i] = num(int64(100 + i))
	}
	return &value.Record{T: d.holderT, Fields: []value.Value{
		dmEntries(ab, num(5), num(6)),
		list(dmEntries(ab, num(7), num(8)), dmEntries(nil), dmEntries(ab[1:], num(9))),
		dmEntries(ab, num(3), num(4)),
		&value.None{},
		&value.Map{Keys: []value.Value{str("x"), str("y")}, Vals: []value.Value{dmEntries(ab, num(11), num(12)), dmEntries(nil)}},
		dmEntries(ab, rec(21), rec(22)),
		&value.Record{T: d.weightsCase, Fields: []value.Value{dmEntries(ab, num(31), num(32))}},
		dmEntries(dmWideIDs(), wideVals...),
		dmGridList(),
		dmNest(),
	}}
}

// dmNest is {k in wide: [Int]} over three keys of three numbers: the dependent level's three entries and the lists' nine numbers reach the array threshold together, where the outer level alone does not.
func dmNest(base ...int64) value.Value {
	lists := make([]value.Value, 3)
	for i := range lists {
		n := int64(1 + 3*i)
		if len(base) > 0 {
			n += base[0]
		}
		lists[i] = list(num(n), num(n+1), num(n+2))
	}
	return dmEntries([]string{"w0", "w3", "w8"}, lists...)
}

// dmGridList is dmGrid maps of {k in rows: Int} of sizes 0 to 2: five elements and four entries, which reach the array threshold only counted together, so they are read from arrays level by level.
func dmGridList() *value.List {
	l := &value.List{}
	for i := range dmGrid {
		keys, vals := []string{"a", "b"}[:i%3], []value.Value{num(int64(i)), num(int64(i + 1))}
		l.Elems = append(l.Elems, dmEntries(keys, vals[:len(keys)]...))
	}
	return l
}

func (d *depmap) fns() []*ir.ExportFn {
	ab := []string{"a", "b"}
	flag := []*ir.Param{{Name: "f", Type: tBool}}
	return []*ir.ExportFn{
		{Name: "totals", Kind: ir.FnPrecomputed, Result: d.byRows, Doc: "The totals by row.", Value: dmEntries(ab, num(41), num(42))},
		lookup("perFlag", "The weights per flag.", d.byRows, flag, dmEntries(ab, num(51), num(52)), dmEntries(ab, num(53), num(54))),
		lookup("nestPerFlag", "The lists per flag.", d.byNest, flag, dmNest(10), dmNest(20)),
	}
}

// depmapPackage is demo.depmap baked: a dependent map `{k in rows: Int}` as a field, a list's element, an optional field, a map's value, a case's field, a bulk one over a long table and a record-valued one, as a selected value and as stored results (CODEGEN.md §4.2, §5.6).
func depmapPackage() *ir.Package { return depmapPackageOf(types.DepMap) }

// depmapPackageOf is depmapPackage with every dependent map of kind: types.Map gives its plain-map twin, written the same.
func depmapPackageOf(kind types.Kind) *ir.Package {
	d := &depmap{kind: kind}
	d.types()
	values := append(d.rows(),
		&ir.Value{Name: "holder", Doc: "The holder.", Type: ir.TypeRef{Kind: types.Record, Named: d.holder}, V: d.holderValue()},
		&ir.Value{Name: "weights", Doc: "The weights.", Type: d.byRows, V: dmEntries([]string{"b", "a"}, num(61), num(62))},
	)
	return &ir.Package{
		Name: dmPkg, Dir: "demo/depmap", Types: []ir.Type{d.row, d.cell, d.pt, d.shape, d.holder}, Values: values, Fns: d.fns(),
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/depmap/out", Mode: ir.ModeBaked, Namespace: "demo::depmap"}},
	}
}
