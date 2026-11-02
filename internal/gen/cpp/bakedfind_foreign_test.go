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

const (
	findBPkg = "find.b"
	findAPkg = "find.a"
)

// foreignFind is the IR of two baked packages: find.b holds the table rows, find.a a keyed list things keyed by `ref b.rows`, which find.a holds as find.b's id enum (CODEGEN.md §5.3, §5.9).
type foreignFind struct {
	row, thing   *ir.Record
	rowT, thingT *types.RecordType
	bEmit, aEmit *ir.Emit
}

func newForeignFind() *foreignFind {
	f := &foreignFind{
		bEmit: &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "find/b", Mode: ir.ModeBaked, Namespace: "find::b"},
		aEmit: &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "find/a", Mode: ir.ModeBaked, Namespace: "find::a"},
	}
	f.row = &ir.Record{Pkg: findBPkg, Name: "Row", Fields: []*ir.Field{field("label", "label", "", tString)}}
	key := tString
	rowRef := ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: findBPkg, Value: "rows", Elem: f.row}}
	f.thing = &ir.Record{Pkg: findAPkg, Name: "Thing", Fields: []*ir.Field{field("row", "row", "", rowRef), field("note", "note", "", tString)}}
	f.rowT, f.thingT = pkgRecordType(findBPkg, "Row", "label"), pkgRecordType(findAPkg, "Thing", "row", "note")
	return f
}

// packageB is find.b: a table whose ids are in no sorted order, wire or position.
func (f *foreignFind) packageB() *ir.Package {
	ids := []string{"zeta", "alpha", "mid", "beta"}
	coll := &types.Collection{Kind: types.CollLet, Pkg: findBPkg, Name: "rows"}
	rows := make([]*value.Record, len(ids))
	for i, id := range ids {
		rows[i] = &value.Record{T: f.rowT, Fields: []value.Value{str(id)}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: id}}}
	}
	elem := ir.TypeRef{Kind: types.Record, Named: f.row}
	return &ir.Package{
		Name: findBPkg, Dir: "find/b", Types: []ir.Type{f.row}, Emits: []*ir.Emit{f.bEmit},
		Values: []*ir.Value{{Name: "rows", IDs: ids, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}, V: &value.Table{Entries: rows}}},
	}
}

// packageA is find.a: things for beta, mid and zeta, not alpha, in an order that is neither the wires' nor the positions'.
func (f *foreignFind) packageA() *ir.Package {
	coll := &types.Collection{Kind: types.CollLet, Pkg: findAPkg, Name: "things"}
	var rows []value.Value
	for _, id := range []string{"mid", "zeta", "beta"} {
		rows = append(rows, &value.Record{T: f.thingT, Fields: []value.Value{key(id), str("note")}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: id}}})
	}
	return &ir.Package{
		Name: findAPkg, Dir: "find/a", Types: []ir.Type{f.thing}, Emits: []*ir.Emit{f.aEmit},
		Imports: []*ir.PackageRef{{Name: findBPkg, Dir: "find/b", Emits: []*ir.Emit{f.bEmit}}},
		Values:  []*ir.Value{findList("things", f.thing, "row", rows[0].(*value.Record), rows[1].(*value.Record), rows[2].(*value.Record))},
	}
}

// foreignFindWant is what testdata/main/baked_find_foreign_main.cpp prints.
const foreignFindWant = "things=3 hits=3 misses=4 failures=0\n"

// CODEGEN.md §5.9, DECISIONS 432: a keyed list keyed by a ref held as the id enum of another package's baked table finds each key at the address At(i) gives and nothing for an id the table lacks or outside its enum, by the wires sorted by bytes through the owner's ToWire (the owner's entry positions are not in this package's IR).
func TestBakedFindForeignIDKeys(t *testing.T) {
	t.Parallel()
	f := newForeignFind()
	dir := t.TempDir()
	var sources []string
	for _, p := range []*ir.Package{f.packageB(), f.packageA()} {
		e := cppEmit(p)
		out := filepath.Join(dir, filepath.FromSlash(e.Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, out, generate(t, p))
		segs := strings.Split(p.Name, ".")
		sources = append(sources, e.Dir+"/"+segs[len(segs)-1]+".gen.cpp")
	}
	copyFile(t, filepath.Join("testdata", "main", "baked_find_foreign_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	for _, out := range buildAndRun(t, dir, append(sources, "main.cpp")) {
		if out != foreignFindWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, foreignFindWant)
		}
	}
}

// CODEGEN.md §5.9, DECISIONS 432: the foreign id key's lookup is declared with the owner's id enum, and defined over the wires sorted by bytes, searched through the owner's ToWire.
func TestBakedFindForeignIDKeyShape(t *testing.T) {
	t.Parallel()
	header, source := generatedTexts(t, newForeignFind().packageA())
	for _, c := range []struct{ file, text string }{
		{header, "    const Thing* Find(::find::b::RowId row) const;\n"},
		{source, "const Thing* Things::Find(::find::b::RowId key) const {\n"},
		{source, "    static constexpr std::array<std::string_view, 3> kKeys = {\"beta\", \"mid\", \"zeta\"};\n"},
		{source, "    static constexpr std::array<uint32_t, 3> kRows = {2, 0, 1};\n"},
		{source, "    const std::string_view wire = ToWire(key);\n"},
		{source, "        if (kKeys[mid] < wire) {\n"},
	} {
		if !strings.Contains(c.file, c.text) {
			t.Errorf("lacks\n%s\nin\n%s", c.text, c.file)
		}
	}
}
