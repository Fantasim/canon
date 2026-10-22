package cppgen_test

import (
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// bakedWant is what testdata/main/baked_main.cpp prints for the baked fixture.
const bakedWant = "open Open col=To do next=taken, retired=0 final=0 rank=2 at=1 reward=0 for=Done\n" +
	"taken Taken col=To do next=closed,open, retired=1 final=0 rank=4 at=-1 reward=1 for=Done\n" +
	"closed Closed col=Done next= retired=0 final=1 rank=6 at=-1 reward=0 for=Done\n" +
	"find=Taken get=Closed id=2 none=0 col=Done\n" +
	"initial=Open key=0 limit=12 tags=2 home=4 prize=7\n" +
	"nextOf=2 maybe=Done none=0 tags=2 primes=3 version=2 doubled=8\n" +
	"points=2 far=9 shelf=2 b=1 crew=5 name=ann pick=done\n" +
	"retired tone=001 live=2 none=0 element=001 live=2 none=0 status=010 live=2 none=0 \n" +
	"failures=0\n"

// CODEGEN.md §5.3, §5.8–§5.10, §7.3, DECISIONS 293: it compiles, its constexpr lookups hold in static_assert and as template arguments, its data reads back.
func TestBakedCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, bakedPackage()), "baked_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"board.gen.cpp", "board_conformance.gen.cpp", "main.cpp"}) {
		if out != bakedWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, bakedWant)
		}
	}
}

// bakedDependentWant is what testdata/main/baked_dependent_main.cpp prints.
const bakedDependentWant = "payload=1:1 multi=1:MI_B=9 deep=0:1 many=1,1:0,1\n"

// CODEGEN.md §5.6, DECISIONS 298: a baked dependent value sits in the branch its record's discriminant selects, a list's elements in its field's, a define key with its value.
func TestBakedDependentCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, bakedDependentPackage()), "baked_dependent_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"demo.gen.cpp", "main.cpp"}) {
		if out != bakedDependentWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, bakedDependentWant)
		}
	}
}

// CODEGEN.md §5.9, DECISIONS 296: a container's Get with an id outside its enum aborts, never reads past its rows.
func TestBakedGetAborts(t *testing.T) {
	t.Parallel()
	compilers, include := cxx.Toolchain(t)
	dir := t.TempDir()
	writeTree(t, dir, generate(t, bakedPackage()), "baked_abort_main.cpp")
	bin := filepath.Join(dir, "abort")
	args := append(append([]string(nil), cxx.Flags...), "-I", dir, "-I", include, "-o", bin,
		filepath.Join(dir, "main.cpp"), filepath.Join(dir, "board.gen.cpp"), filepath.Join(dir, "board_conformance.gen.cpp"))
	if out, err := compile(compilers[0], args, dir); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	err := exec.Command(bin).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.Exited() {
		t.Errorf("run: %v, want an abort", err)
	}
}

// bakedBulkWant is what testdata/main/baked_bulk_main.cpp prints for the baked-bulk fixture.
const bakedBulkWant = "i0 retired=0 Item 0 code=100 tag=t0 tier=t0 note=none next=Item 1 coins=-1 twice=0 bonus=0,7\n" +
	"i1 retired=1 Item 1 code=101 tag=t1 tier=t1 note=odd 1 next=Item 2 coins=10 twice=2 bonus=8,15\n" +
	"i2 retired=0 Item 2 code=102 tag=t2 tier=t2 note=none next=Item 3 coins=-1 twice=4 bonus=16,23\n" +
	"i3 retired=0 Item 3 code=103 tag=t3 tier=t3 note=odd 3 next=Item 4 coins=30 twice=6 bonus=24,31\n" +
	"i4 retired=0 Item 4 code=104 tag=t4 tier=t4 note=none next=Item 5 coins=-1 twice=8 bonus=32,39\n" +
	"i5 retired=0 Item 5 code=105 tag=t5 tier=t5 note=odd 5 next=Item 6 coins=50 twice=10 bonus=40,47\n" +
	"i6 retired=0 Item 6 code=106 tag=t6 tier=t6 note=none next=Item 7 coins=-1 twice=12 bonus=48,55\n" +
	"i7 retired=0 Item 7 code=107 tag=t7 tier=t7 note=odd 7 next=Item 0 coins=70 twice=14 bonus=56,63\n" +
	"byCode=Item 5 names=8 name \"0\"|name \"7\" odd=7:9,0,55,127\n" +
	"weights=8 w7=49 picks=8 Item 7 Item 0 keys=i7\n" +
	"crews=8 crew30=crew 3 label2=Tier 2 label3=none\n" +
	"names=8 T0 T7 ranks=8 t1=7\n" +
	"i0 ratio=-0 neg=1 weight=0.5 big=-9223372036854775808 wait=-1 opts=1: none\n" +
	"i1 ratio=0.25 neg=0 weight=1.5 big=1 wait=1000 opts=2: o none\n" +
	"i2 ratio=0.5 neg=0 weight=2.5 big=2 wait=-1 opts=2: o none\n" +
	"i3 ratio=0.75 neg=0 weight=3.5 big=3 wait=3000 opts=2: o none\n" +
	"i4 ratio=1 neg=0 weight=4.5 big=4 wait=-1 opts=2: o none\n" +
	"i5 ratio=1.25 neg=0 weight=5.5 big=5 wait=5000 opts=2: o none\n" +
	"i6 ratio=1.5 neg=0 weight=6.5 big=6 wait=-1 opts=2: o none\n" +
	"i7 ratio=1.75 neg=0 weight=7.5 big=9223372036854775807 wait=7000 opts=0:\n" +
	"grid=3: [123] [] [4567]\n" +
	"groups=3: g0=ab g1= g2=cde\n" +
	"constGroups=8: c0=0 c1=1 c2=2 c3=0 c4=1 c5=2 c6=0 c7=1 c5[0]=3:1\n" +
	"constGrid=3:0,5,0 last=5\n"

// DECISIONS 392, CODEGEN.md §5.1, §5.8–§5.10: every construct read from arrays compiles in every mode and reads its data back.
func TestBakedBulkCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, bulkSmallPackage()), "baked_bulk_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"bulk.gen.cpp", "main.cpp"}) {
		if out != bakedBulkWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, bakedBulkWant)
		}
	}
}

// nulWant is what testdata/main/nul_main.cpp prints: every path keeps the three bytes.
const nulWant = "field=3:1\nshort=3:1\nlong=3:1\nshortKey=3:1\nlongKey=3:1\nfind=1\n" +
	"const=3:1\nconstShort=3:1\nconstLong=3:1\ncell=3:1\n"

// CODEGEN.md §3.4, DECISIONS 392: a string holding a NUL is read back whole on every path, inline or from an array.
func TestBakedNulCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, nulPackage()), "nul_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"nulstr.gen.cpp", "main.cpp"}) {
		if out != nulWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, nulWant)
		}
	}
}

// nulTypesWant is what testdata/main/nul_types_main.cpp prints.
const nulTypesWant = "whole: ok amount=5\ncut: Holder: $.prize.kind: unknown case a amount=-1\nwire=3\n"

// CODEGEN.md §3.4, §5.13, DECISIONS 392: a decoder compares a case wire holding a NUL whole.
func TestTypesNulCaseWireCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, nulTypesPackage()), "nul_types_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"nulstr.gen.cpp", "main.cpp"}) {
		if out != nulTypesWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, nulTypesWant)
		}
	}
}
