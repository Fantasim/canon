package cppgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

const (
	// bakedChainEnumWant is what testdata/main/baked_chainenum_main.cpp prints.
	bakedChainEnumWant = "tones=3 picks=2 over=2 ranks=3 rpicks=3 rover=3\nhits=16 misses=8 failures=0\n"
	// bakedChainForeignWant is what testdata/main/baked_chainforeign_main.cpp prints.
	bakedChainForeignWant = "byRow=3 over=2\nhits=5 misses=9 failures=0\n"
)

// CODEGEN.md §5.9, §7.3, DECISIONS 432, 456: a baked container keyed by a ref into a list keyed by a ref into a list keyed by an enum is searched in the enum's order and built with its members, and finds every row.
func TestBakedFindNestedRefEnumLeaf(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, chainEnumPackage()), "baked_chainenum_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"chainenum.gen.cpp", "main.cpp"}) {
		if out != bakedChainEnumWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, bakedChainEnumWant)
		}
	}
}

// CODEGEN.md §5.3, §5.9, DECISIONS 432, 456: a baked container keyed by a ref into a list keyed by a ref to another package's table is held as that table's id enum and searched by wire through the owner's ToWire, and built with the enum's members, down the chain.
func TestBakedFindNestedRefForeignLeaf(t *testing.T) {
	t.Parallel()
	c := newChainForeign()
	dir := t.TempDir()
	var sources []string
	for _, p := range []*ir.Package{c.packageB(), c.packageA()} {
		e := cppEmit(p)
		out := filepath.Join(dir, filepath.FromSlash(e.Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, out, generate(t, p))
		segs := strings.Split(p.Name, ".")
		sources = append(sources, e.Dir+"/"+segs[len(segs)-1]+".gen.cpp")
	}
	copyFile(t, filepath.Join("testdata", "main", "baked_chainforeign_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	for _, out := range buildAndRun(t, dir, append(sources, "main.cpp")) {
		if out != bakedChainForeignWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, bakedChainForeignWant)
		}
	}
}
