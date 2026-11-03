package cppgen_test

import "testing"

// bakedNestRefWant is what testdata/main/baked_nestref_main.cpp prints for the nested-ref fixture.
const bakedNestRefWant = "byKind=3 outer=3 deep=3\nhits=9 misses=6 failures=0\n"

// CODEGEN.md §5.9, §7.3, DECISIONS 432, 456: a baked container keyed by a ref into a keyed list that is itself keyed by a ref into a table is searched in the table id's entry order, its keys written as that id enum's members, whatever the depth of the chain.
func TestBakedFindNestedRefKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, chainRefPackage()), "baked_nestref_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"nestref.gen.cpp", "main.cpp"}) {
		if out != bakedNestRefWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, bakedNestRefWant)
		}
	}
}
