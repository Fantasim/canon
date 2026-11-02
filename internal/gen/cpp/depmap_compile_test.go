package cppgen_test

import (
	"bytes"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// depmapWant is what testdata/main/depmap_main.cpp prints.
const depmapWant = "m=2: a=5 b=6\n" +
	"many=3\n" +
	"  many=2: a=7 b=8\n" +
	"  many=0:\n" +
	"  many=1: b=9\n" +
	"opt=1 absent=0\n" +
	"opt=2: a=3 b=4\n" +
	"byName=2\n" +
	"  x=2: a=11 b=12\n" +
	"  y=0:\n" +
	"recs=2: a=21 b=22\n" +
	"shape=2: a=31 b=32\n" +
	"big=9 w7=107 first=w0 last=w8\n" +
	"grid=5: [] [a1] [a2b3] [] [a4]\n" +
	"nest: w0=1,2,3; w3=4,5,6; w8=7,8,9;\n" +
	"nestPerFlag(false): w0=11,12,13; w3=14,15,16; w8=17,18,19;\n" +
	"nestPerFlag(true): w0=21,22,23; w3=24,25,26; w8=27,28,29;\n" +
	"weights=2: b=61 a=62\n" +
	"totals=2: a=41 b=42\n" +
	"perFlag(false)=2: a=51 b=52\n" +
	"perFlag(true)=2: a=53 b=54\n" +
	"scores a=2: a=10 b=11\n" +
	"scores b=2: a=20 b=21\n" +
	"cell w3 links=2: a=3 b=6\n" +
	"cell w3 hops=1: b=9\n" +
	"cell w8 links=2: a=8 b=16\n" +
	"cell w8 hops=1: b=24\n" +
	"find a=62\n"

// CODEGEN.md §4.2, DECISIONS 392: a dependent map is written exactly as its plain-map twin, so a literal whose dependent levels reach the array threshold (a list of small ones, a map of lists, a lookup's cells) is read from arrays, never written inline in Build.
func TestBakedDependentMapEqualsPlainTwin(t *testing.T) {
	t.Parallel()
	dep, plain := generate(t, depmapPackageOf(types.DepMap)), generate(t, depmapPackageOf(types.Map))
	if len(dep) != len(plain) {
		t.Fatalf("%d files, twin %d", len(dep), len(plain))
	}
	for i, f := range dep {
		if f.Path != plain[i].Path || !bytes.Equal(f.Content, plain[i].Content) {
			t.Errorf("%s differs from the plain-map twin's", f.Path)
		}
	}
}

// CODEGEN.md §4.2, §5.6: a baked dependent map `{k in t: R}` is written as a map of t's key, in every position a map may sit (a field, a list's element, an optional, a map's value, a case's field, a bulk one, a selected value, stored results), and reads back the Canon values.
func TestBakedDependentMapCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, depmapPackage()), "depmap_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"depmap.gen.cpp", "main.cpp"}) {
		if out != depmapWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, depmapWant)
		}
	}
}
