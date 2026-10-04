package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const boardPkg = "demo.board"

// board is the IR of the baked fixture, shared by its builders.
type board struct {
	tone, element   *ir.Enum
	column, point   *ir.Record
	status          *ir.Record
	reward          *ir.Variant
	columnT, pointT *types.RecordType
	statusT         *types.RecordType
	coins, nothing  *types.CaseType
	crew            *ir.Record
}

// boardRef is a ref into table value of this package, keyed by String as stage E types it.
func boardRef(table string, elem ir.Type) ir.TypeRef {
	key := tString
	return ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: boardPkg, Value: table, Elem: elem}}
}

// recordType is the checker's record type of the field names given: what a value's T names.
func recordType(name string, fields ...string) *types.RecordType {
	rt := &types.RecordType{Pkg: boardPkg, Name: name}
	for i, f := range fields {
		rt.Fields = append(rt.Fields, &types.Field{Name: f, Index: i})
	}
	return rt
}

func member(i int) *value.Member         { return &value.Member{Index: i} }
func key(s string) *value.Ref            { return &value.Ref{Key: value.Key{S: s}} }
func list(vs ...value.Value) *value.List { return &value.List{Elems: vs} }

// row is entry id of table, retired or not.
func row(t *types.RecordType, table, id string, retired bool, fields ...value.Value) *value.Record {
	coll := &types.Collection{Kind: types.CollLet, Pkg: boardPkg, Name: table}
	return &value.Record{T: t, Fields: fields, Ident: &value.Identity{Coll: coll, Key: value.Key{S: id}, Retired: retired}}
}

// bakedDependentPackage is dependentPackage baked, its event's dependent values set (CODEGEN.md §5.6).
func bakedDependentPackage() *ir.Package {
	p := dependentPackage()
	p.Emits[0].Mode = ir.ModeBaked
	et := recordType("EventType", "param")
	event := recordType("Event", "active", "payload", "kind3", "multi", "et", "deep", "many")
	p.Values[0].V = &value.Record{T: event, Fields: []value.Value{
		boolean(true), member(1), member(1), key("MI_B"), &value.Record{T: et, Fields: []value.Value{member(0)}},
		boolean(true), list(member(0), member(1)),
	}}
	return p
}

// bakedPackage is a baked emit of CODEGEN.md §5.3, §5.9, §5.10's constructs, constexpr lookups of every result kind included (decision 293).
func bakedPackage() *ir.Package {
	b := &board{}
	b.types()
	p := &ir.Package{
		Name: boardPkg, Dir: "demo/board",
		Types:  []ir.Type{b.tone, b.element, b.column, b.point, b.reward, b.status},
		Values: b.values(),
		Fns:    b.packageFns(),
		Emits:  []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/board/out", Mode: ir.ModeBaked, Namespace: "demo::board"}},
	}
	b.extras(p)
	return p
}

func (b *board) types() {
	b.tone = &ir.Enum{Pkg: boardPkg, Name: "Tone", Doc: "How loud a message is.", Members: []*ir.EnumMember{
		{Name: "quiet", Wire: "quiet", Index: 0}, {Name: "loud", Wire: "loud", Index: 1}, {Name: "old", Wire: "old", Index: 2, Retired: true},
	}}
	u8 := ir.TypeRef{Kind: types.Int, Bits: 8}
	b.element = &ir.Enum{Pkg: boardPkg, Name: "Element", Codes: &u8, Members: []*ir.EnumMember{
		{Name: "FIRE", Wire: "FIRE", Index: 0, Code: 1}, {Name: "WATER", Wire: "WATER", Index: 1, Code: 2},
		{Name: "EARTH", Wire: "EARTH", Index: 2, Code: 3, Retired: true},
	}}
	b.column = &ir.Record{Pkg: boardPkg, Name: "Column", Doc: "A column of the board.", Fields: []*ir.Field{field("title", "title", "Its title.", tString)}}
	b.point = &ir.Record{Pkg: boardPkg, Name: "Point", Fields: []*ir.Field{field("x", "x", "", tInt), field("y", "y", "", tInt)}}
	b.reward = &ir.Variant{Pkg: boardPkg, Name: "Reward", Doc: "What a status gives.", Tag: "kind", Cases: []*ir.Case{
		{Name: "coins", Wire: "coins", Fields: []*ir.Field{field("amount", "amount", "How many.", tInt)}},
		{Name: "nothing", Wire: "nothing"},
	}}
	b.columnT, b.pointT = recordType("Column", "title"), recordType("Point", "x", "y")
	b.coins = &types.CaseType{Name: "coins", Index: 0, Fields: []*types.Field{{Name: "amount"}}}
	b.nothing = &types.CaseType{Name: "nothing", Index: 1}
	b.status = &ir.Record{Pkg: boardPkg, Name: "Status", Doc: "One point of the lifecycle."}
	at := field("at", "at", "Where it sits.", ir.TypeRef{Kind: types.Record, Named: b.point})
	at.Optional = true
	b.status.Fields = []*ir.Field{
		field("label", "label", "Its label.", tString),
		field("column", "column", "Where it is shown.", boardRef("columns", b.column)),
		field("next", "next", "The statuses it may move to.", listOf(boardRef("statuses", b.status))),
		field("reward", "reward", "", ir.TypeRef{Kind: types.Variant, Named: b.reward}),
		at, field("weight", "weight", "", tInt),
	}
	b.statusT = recordType("Status", "label", "column", "next", "reward", "at", "weight")
	tTone := ir.TypeRef{Kind: types.Enum, Named: b.tone}
	b.status.Methods = []*ir.ExportFn{
		{Name: "final", Kind: ir.FnPrecomputed, Result: tBool, Doc: "Whether nothing follows it."},
		{Name: "rank", Kind: ir.FnLookup, Result: tInt, Params: []*ir.Param{{Name: "t", Type: tTone}}},
		{Name: "columnFor", Kind: ir.FnLookup, Result: boardRef("columns", b.column), Params: []*ir.Param{{Name: "s", Type: boardRef("statuses", b.status)}}},
	}
}

// statusRows are the entries of statuses: taken is retired, and each holds its stored results.
func (b *board) statusRows() []*value.Record {
	open := row(b.statusT, "statuses", "open", false, str("Open"), key("todo"), list(key("taken")),
		&value.Record{T: b.coins, Fields: []value.Value{num(2)}}, &value.Record{T: b.pointT, Fields: []value.Value{num(1), num(2)}}, num(1))
	taken := row(b.statusT, "statuses", "taken", true, str("Taken"), key("todo"), list(key("closed"), key("open")),
		&value.Record{T: b.nothing}, &value.None{}, num(1))
	closed := row(b.statusT, "statuses", "closed", false, str("Closed"), key("done"), list(),
		&value.Record{T: b.coins, Fields: []value.Value{num(5)}}, &value.None{}, num(3))
	rows := []*value.Record{open, taken, closed}
	final, rank, columnFor := b.status.Methods[0], b.status.Methods[1], b.status.Methods[2]
	toneDomain := [][]value.Value{{member(0), member(1), member(2)}}
	idDomain := [][]value.Value{{key("open"), key("taken"), key("closed")}}
	for i, r := range rows {
		w := int64(i + 1)
		final.Instances = append(final.Instances, &ir.Instance{Recv: r, Result: boolean(i == len(rows)-1)})
		rank.Instances = append(rank.Instances, &ir.Instance{Recv: r, Table: &ir.LookupTable{Domains: toneDomain, Cells: []value.Value{num(w), num(2 * w), num(w)}}})
		columnFor.Instances = append(columnFor.Instances, &ir.Instance{Recv: r, Table: &ir.LookupTable{Domains: idDomain, Cells: []value.Value{key("todo"), key("done"), key("done")}}})
	}
	return rows
}

func (b *board) values() []*ir.Value {
	colElem := ir.TypeRef{Kind: types.Record, Named: b.column}
	statusElem := ir.TypeRef{Kind: types.Record, Named: b.status}
	columns := &value.Table{Entries: []*value.Record{
		row(b.columnT, "columns", "todo", false, str("To do")), row(b.columnT, "columns", "done", false, str("Done")),
	}}
	return []*ir.Value{
		{Name: "columns", Doc: "The columns.", IDs: []string{"todo", "done"}, Type: ir.TypeRef{Kind: types.Table, Elem: &colElem}, V: columns},
		{Name: "statuses", Doc: "The statuses.", IDs: []string{"open", "taken", "closed"}, Type: ir.TypeRef{Kind: types.Table, Elem: &statusElem}, V: &value.Table{Entries: b.statusRows()}},
		{Name: "initialStatus", Doc: "Where a new post starts.", Type: boardRef("statuses", b.status), V: key("open")},
		{Name: "limit", Doc: "The most posts.", Type: tInt, V: num(12)},
		{Name: "tags", Type: listOf(tString), V: list(str("a"), str("b"))},
		{Name: "home", Doc: "The board's corner.", Type: ir.TypeRef{Kind: types.Record, Named: b.point}, V: &value.Record{T: b.pointT, Fields: []value.Value{num(3), num(4)}}},
		{Name: "prize", Type: ir.TypeRef{Kind: types.Variant, Named: b.reward}, V: &value.Record{T: b.coins, Fields: []value.Value{num(7)}}},
	}
}

// lookup is a package lookup over params whose cells are given in domain order.
func lookup(name, doc string, result ir.TypeRef, params []*ir.Param, cells ...value.Value) *ir.ExportFn {
	return &ir.ExportFn{Name: name, Doc: doc, Kind: ir.FnLookup, Result: result, Params: params, Table: &ir.LookupTable{Cells: cells}}
}

func (b *board) packageFns() []*ir.ExportFn {
	tTone, tElement := ir.TypeRef{Kind: types.Enum, Named: b.tone}, ir.TypeRef{Kind: types.Enum, Named: b.element}
	byTone, byElement := []*ir.Param{{Name: "t", Type: tTone}}, []*ir.Param{{Name: "e", Type: tElement}}
	byStatus := []*ir.Param{{Name: "s", Type: boardRef("statuses", b.status)}}
	colRef := boardRef("columns", b.column)
	optCol := ir.TypeRef{Kind: types.Optional, Elem: &colRef}
	versionOf := lookup("versionOf", "The version an element was added in.", tInt, byElement, num(1), num(2), num(2))
	doubled := &ir.ExportFn{
		Name: "doubled", File: "board.canon", Kind: ir.FnTranslated, Result: tInt, Order: 1,
		Params: []*ir.Param{{Name: "e", Type: tElement}, {Name: "n", Type: tInt}},
		Body:   &ir.Binary{T: tInt, Op: ir.OpMul, X: &ir.CallFn{T: tInt, Fn: versionOf, Args: []ir.PExpr{&ir.ParamRef{T: tElement, Index: 0}}}, Y: &ir.ParamRef{T: tInt, Index: 1}},
		Vectors: []*ir.Vector{
			vec(nil, []value.Value{member(0), num(3)}, num(3), ""),
			vec(nil, []value.Value{member(1), num(4)}, num(8), ""),
		},
	}
	return []*ir.ExportFn{
		versionOf,
		lookup("labelOf", "A tone's label.", tString, byTone, str("Quiet"), str("Loud"), str("Quiet")),
		lookup("isOpen", "Whether a status is open.", tBool, byStatus, boolean(true), boolean(false), boolean(false)),
		lookup("toneOf", "", tTone, byElement, member(1), member(0), member(0)),
		lookup("columnOf", "A status's column.", colRef, byStatus, key("todo"), key("todo"), key("done")),
		lookup("delayOf", "", tDuration, []*ir.Param{{Name: "b", Type: tBool}}, dur(500), dur(2000)),
		lookup("nextOf", "The statuses after one.", listOf(boardRef("statuses", b.status)), byStatus,
			list(key("taken")), list(key("closed"), key("open")), list()),
		lookup("maybeColumn", "A tone's column, if any.", optCol, byTone, &value.None{}, key("done"), &value.None{}),
		lookup("tagsOf", "", listOf(tString), byTone, list(str("q")), list(str("l"), str("L")), list()),
		{Name: "greeting", Doc: "The greeting.", Kind: ir.FnPrecomputed, Result: tString, Value: str("hi \"you\"")},
		{Name: "primes", Kind: ir.FnPrecomputed, Result: listOf(tInt), Value: list(num(2), num(3), num(5))},
		doubled,
	}
}
