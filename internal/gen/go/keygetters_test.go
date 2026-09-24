package gogen_test

import (
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const lookupsPkg = "demo.lookups"

// lookupTable is a baked table of a one-field record, its entries keyed by ids.
func lookupTable(name, coll string, ids ...string) (*ir.Record, *ir.Value, []*value.Record) {
	rec := &ir.Record{Pkg: lookupsPkg, Name: name, Fields: []*ir.Field{wired("label", "label", "", strT)}}
	twin := &types.RecordType{Pkg: lookupsPkg, Name: name, Fields: []*types.Field{{Name: "label"}}}
	ident := &types.Collection{Kind: types.CollLet, Pkg: lookupsPkg, Name: coll}
	tbl := &value.Table{}
	for _, id := range ids {
		tbl.Entries = append(tbl.Entries, &value.Record{T: twin, Fields: []value.Value{&value.Str{V: id}}, Ident: &value.Identity{Coll: ident, Key: value.Key{S: id}}})
	}
	elem := typed(rec, types.Record)
	return rec, &ir.Value{Name: coll, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}, IDs: ids, V: tbl}, tbl.Entries
}

// bakedLookups has lookup methods with ref results: resolved (next, peers) and key only
// (other, into a table the emit leaves out).
func bakedLookups() *ir.Package {
	status, statuses, rows := lookupTable("Status", "statuses", "a", "b")
	other, others, _ := lookupTable("Other", "others", "x")
	loud := []*ir.Param{{Name: "loud", Type: boolT}}
	ref, key := refT(lookupsPkg, "statuses", status, false), refT(lookupsPkg, "others", other, false)
	next := &ir.ExportFn{Name: "next", Kind: ir.FnLookup, Result: optT(ref), Params: loud, Doc: "The status after this one."}
	peers := &ir.ExportFn{Name: "peers", Kind: ir.FnLookup, Result: listT(ref), Params: loud}
	far := &ir.ExportFn{Name: "other", Kind: ir.FnLookup, Result: key, Params: loud, Doc: "Key only."}
	status.Methods = []*ir.ExportFn{next, peers, far}
	domain := [][]value.Value{{&value.Bool{}, &value.Bool{V: true}}}
	r := func(k string) value.Value { return &value.Ref{Key: value.Key{S: k}} }
	list := func(ks ...string) value.Value {
		l := &value.List{}
		for _, k := range ks {
			l.Elems = append(l.Elems, r(k))
		}
		return l
	}
	for i, row := range rows {
		self, peer := []string{"a", "b"}[i], []string{"b", "a"}[i]
		next.Instances = append(next.Instances, &ir.Instance{Recv: row, Table: &ir.LookupTable{Domains: domain, Cells: []value.Value{&value.None{}, r(peer)}}})
		peers.Instances = append(peers.Instances, &ir.Instance{Recv: row, Table: &ir.LookupTable{Domains: domain, Cells: []value.Value{list(), list(peer, self)}}})
		far.Instances = append(far.Instances, &ir.Instance{Recv: row, Table: &ir.LookupTable{Domains: domain, Cells: []value.Value{r("x"), r("x")}}})
	}
	e := goData("demo/lookups", "lookups")
	e.Mode, e.Values = ir.ModeBaked, []string{"statuses"}
	return &ir.Package{Name: lookupsPkg, Dir: "demo/lookups", Types: []ir.Type{status, other}, Values: []*ir.Value{statuses, others}, Emits: []*ir.Emit{e}}
}

// Log-2026-09-24 (gen/go review calls), CODEGEN.md §5.8, §5.10: a lookup method's ref result has its key getter in baked mode too.
func TestBakedLookupKeysGolden(t *testing.T) {
	files := generateData(t, bakedLookups())
	checkGoldens(t, files, sortedPaths(files), filepath.Join("testdata", "lookups"))
}

// CODEGEN.md §9: the key getters compile and read the keys of the resolved entries.
func TestBakedLookupKeysRun(t *testing.T) {
	runData(t, generateData(t, bakedLookups()), "demo/lookups/out/go", "testdata/smoke/lookups_test.go", nil)
}
