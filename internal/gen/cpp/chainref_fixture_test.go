package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const chainPkg = "demo.nestref"

// chainColl is the collection of value table of package pkg, a let of it.
func chainColl(pkg, table string) *types.Collection {
	return &types.Collection{Kind: types.CollLet, Pkg: pkg, Name: table}
}

// chainRef is a ref to value table of package pkg keyed by key: into a keyed list when keyed, else into a table.
func chainRef(pkg, table string, key ir.TypeRef, keyed bool) ir.TypeRef {
	return ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: pkg, Value: table, Keyed: keyed}}
}

// chainRow is the row of keyed list name of package pkg keyed by id, its fields given.
func chainRow(pkg, name string, rec *ir.Record, id string, fields ...value.Value) value.Value {
	return &value.Record{T: bulkTypeOf(pkg, rec.Name, rec), Fields: fields, Ident: &value.Identity{Coll: chainColl(pkg, name), Key: value.Key{S: id}}}
}

// chainListOf is the keyed list name of rows of rec keyed by its first field, the rows given.
func chainListOf(name string, rec *ir.Record, rows ...value.Value) *ir.Value {
	elem := ir.TypeRef{Kind: types.Record, Named: rec}
	by := rec.Fields[0].Name
	return &ir.Value{Name: name, Type: ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: by, WirePath: []string{by}}}, V: list(rows...)}
}

// chainList is the keyed list name of package pkg whose rows hold a ref to each id of ids, in the order given.
func chainList(pkg, name string, rec *ir.Record, ids []string) *ir.Value {
	rows := make([]value.Value, len(ids))
	for i, id := range ids {
		rows[i] = chainRow(pkg, name, rec, id, key(id), str("l"))
	}
	return chainListOf(name, rec, rows...)
}

// chainRefPackage is demo.nestref, a baked package whose keyed lists are keyed by a ref into a keyed list that is itself keyed by a ref into a table, three refs deep for the last one (CODEGEN.md §5.9, DECISIONS 432, 456: a ref into a keyed list sorts by that list's key, recursively).
func chainRefPackage() *ir.Package {
	label := field("label", "label", "", tString)
	kind := &ir.Record{Pkg: chainPkg, Name: "Kind", Fields: []*ir.Field{label}}
	kindRef := chainRef(chainPkg, "kinds", tString, false)
	byKindRef := chainRef(chainPkg, "byKind", kindRef, true)
	outerRef := chainRef(chainPkg, "outer", byKindRef, true)
	kindRow := &ir.Record{Pkg: chainPkg, Name: "KindRow", Fields: []*ir.Field{field("kind", "kind", "", kindRef), label}}
	outerRow := &ir.Record{Pkg: chainPkg, Name: "OuterRow", Fields: []*ir.Field{field("outer", "outer", "", byKindRef), label}}
	deepRow := &ir.Record{Pkg: chainPkg, Name: "DeepRow", Fields: []*ir.Field{field("deep", "deep", "", outerRef), label}}
	kindRef.Ref.Elem, byKindRef.Ref.Elem, outerRef.Ref.Elem = kind, kindRow, outerRow
	ids := []string{"zeta", "alpha", "mid", "beta"}
	kinds := make([]*value.Record, len(ids))
	for i, id := range ids {
		kinds[i] = chainRow(chainPkg, "kinds", kind, id, str(id)).(*value.Record)
	}
	kindElem := ir.TypeRef{Kind: types.Record, Named: kind}
	return &ir.Package{
		Name: chainPkg, Dir: "demo/nestref",
		Types: []ir.Type{kind, kindRow, outerRow, deepRow},
		Values: []*ir.Value{
			{Name: "kinds", IDs: ids, Type: ir.TypeRef{Kind: types.Table, Elem: &kindElem}, V: &value.Table{Entries: kinds}},
			chainList(chainPkg, "byKind", kindRow, []string{"mid", "zeta", "beta"}),
			chainList(chainPkg, "outer", outerRow, []string{"beta", "mid", "zeta"}),
			chainList(chainPkg, "deep", deepRow, []string{"zeta", "beta", "mid"}),
		},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/nestref/out", Mode: ir.ModeBaked, Namespace: "demo::nestref"}},
	}
}
