package cppgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// emptyOwner is e.b, baked, whose table items is empty and whose Band's lookup bonus takes a ref into it; e.a, in data mode, reads a Band (CODEGEN.md §5.10: an empty domain).
func emptyOwner() (owner, reader *ir.Package) {
	ownerEmit := &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "e/b/out", Mode: ir.ModeBaked, Namespace: "e::b"}
	item := &ir.Record{Pkg: "e.b", Name: "Item", Fields: []*ir.Field{field("rank", "rank", "", tInt)}}
	itemElem := ir.TypeRef{Kind: types.Record, Named: item}
	key := tString
	itemRef := ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: "e.b", Value: "items", Elem: item}}
	bonus := &ir.ExportFn{Name: "bonus", Kind: ir.FnLookup, Result: tInt, Params: []*ir.Param{{Name: "t", Type: itemRef}}, Domains: [][]value.Value{{}}}
	band := &ir.Record{Pkg: "e.b", Name: "Band", Fields: []*ir.Field{field("base", "base", "", tInt)}, Methods: []*ir.ExportFn{bonus}}
	owner = &ir.Package{
		Name: "e.b", Dir: "e/b", Types: []ir.Type{item, band}, Emits: []*ir.Emit{ownerEmit},
		Values: []*ir.Value{{Name: "items", Type: ir.TypeRef{Kind: types.Table, Elem: &itemElem}, V: &value.Table{}}},
	}
	zone := &ir.Record{Pkg: "e.a", Name: "Zone", Fields: []*ir.Field{field("band", "band", "", ir.TypeRef{Kind: types.Record, Named: band})}}
	zoneT, bandT := pkgRecordType("e.a", "Zone", "band"), pkgRecordType("e.b", "Band", "base")
	b := &value.Record{T: bandT, Fields: []value.Value{num(3)}}
	bonus.Instances = []*ir.Instance{{Recv: b, Table: &ir.LookupTable{Domains: [][]value.Value{{}}}}}
	reader = &ir.Package{
		Name: "e.a", Dir: "e/a", Types: []ir.Type{zone},
		Values:  []*ir.Value{{Name: "zone", Schema: "e.a.Zone@00000004", Type: ir.TypeRef{Kind: types.Record, Named: zone}, V: &value.Record{T: zoneT, Fields: []value.Value{b}}}},
		Emits:   []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "e/a/out", Mode: ir.ModeData, Namespace: "e::a"}},
		Imports: []*ir.PackageRef{{Name: "e.b", Dir: "e/b", Emits: []*ir.Emit{ownerEmit}}},
	}
	return owner, reader
}

func emptyOwnerB() *ir.Package { b, _ := emptyOwner(); return b }

func emptyOwnerA() *ir.Package { _, a := emptyOwner(); return a }

// CODEGEN.md §5.10, §2.8: a reader of another package's record whose lookup takes a ref into the owner's empty table reads `$bonus` without a cell, and compiles under every §9 compiler.
func TestEmptyOwnerDomainCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var sources []string
	for _, p := range []*ir.Package{emptyOwnerB(), emptyOwnerA()} {
		e := cppEmit(p)
		out := filepath.Join(dir, filepath.FromSlash(e.Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, out, generate(t, p))
		segs := strings.Split(p.Name, ".")
		sources = append(sources, e.Dir+"/"+segs[len(segs)-1]+".gen.cpp")
	}
	copyFile(t, filepath.Join("testdata", "main", "empty_owner_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	copyFile(t, filepath.Join("testdata", "main", "empty_owner", "zone.json"), filepath.Join(dir, "zone.json"), same)
	for _, out := range buildAndRun(t, dir, append(sources, "main.cpp"), dir) {
		if !strings.HasSuffix(out, "failures: 0\n") {
			t.Errorf("driver output:\n%s", out)
		}
	}
}
