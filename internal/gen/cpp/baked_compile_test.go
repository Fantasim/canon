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
