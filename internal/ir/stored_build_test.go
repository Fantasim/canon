package ir_test

import (
	"bytes"
	"context"
	"go/version"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/testkit/tsc"
)

const (
	e2eStored      = "testdata/e2e/stored/*.txtar"
	e2eChain       = "testdata/e2e/chain.txtar"
	e2eStoredJSON  = "json.golden"
	e2eJSONSuffix  = ".json"
	e2eTSSuffix    = ".ts"
	e2eTSNoEmit    = "--noEmit"
	e2eGoModPrefix = "module "
	e2eGoModGo     = "\n\ngo "
	e2eGoModFile   = "go.mod"
)

// TestStoredResultsBuild is WIRE.md §5.11 (WIR-09) and decision 128 end to end: a stored export fn whose result is a record or case with stored export fns of its own (a method, a lookup cell, a package fn, a case's method, another package's record) gets their `$` keys, because stage E precomputes them on the result (EVALUATION.md §2.3, decision 194); `canon build` then meets no generator refusal (decision 37), json.golden is every json file it writes, and its Go vets, its C++ and its TypeScript compile.
func TestStoredResultsBuild(t *testing.T) {
	golden.Run(t, e2eStored, func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		dir := t.TempDir()
		writeProject(t, dir, c.Path)
		p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Build(context.Background(), build.BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out := renderFindings(t, res.Findings); res.Summary.Errors > 0 {
			t.Fatalf("build findings:\n%s", out)
		}
		gen := filepath.Join(dir, e2eGenRoot)
		if !testing.Short() {
			compileStored(t, gen)
		}
		return storedJSON(t, gen)
	}, golden.Expected(e2eStoredJSON))
}

// TestStoredResultRefusedAtCheck is DECISIONS 284 and CODEGEN.md §2.7 through internal/build: a stored result that always holds a fresh receiver of its own record is E8019 from `canon check`, so stage E never precomputes it (an endless chain before); it raises no internal error, and `canon build` stops at the same one finding. A stored result reaching a field cycle that an optional breaks builds (log-2026-10-06 "U5 review FAIL"; e2e/lifted/cpp_cycle_result_order_q.txtar).
func TestStoredResultRefusedAtCheck(t *testing.T) {
	for _, tc := range []struct{ fixture, place string }{
		{e2eChain, "q/q.canon:11:13"},
	} {
		dir := t.TempDir()
		writeProject(t, dir, tc.fixture)
		p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Check(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if out := renderFindings(t, res.Findings); !oneCycleAt(res.Findings, out, tc.place) {
			t.Errorf("%s: want one %s at %s from check, got:\n%s", tc.fixture, diag.E8019.Def().Code, tc.place, out)
		}
		built, err := p.Build(context.Background(), build.BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if out := renderFindings(t, built.Findings); !oneCycleAt(built.Findings, out, tc.place) {
			t.Errorf("%s: want the same one finding from build, got:\n%s", tc.fixture, out)
		}
	}
}

// oneCycleAt tells that f, rendered out, is one cycle finding at place.
func oneCycleAt(f build.Findings, out, place string) bool {
	return len(f.List) == 1 && f.List[0].Code == diag.E8019.Def().Code && strings.Contains(out, place+"\n")
}

// compileStored vets the Go, and compiles the C++ and the TypeScript, generated under gen.
func compileStored(t *testing.T, gen string) {
	t.Helper()
	mod := e2eGoModPrefix + e2eGoModule + e2eGoModGo + strings.TrimPrefix(version.Lang(runtime.Version()), "go") + "\n"
	if err := os.WriteFile(filepath.Join(gen, e2eGoModFile), []byte(mod), e2eFileMode); err != nil {
		t.Fatal(err)
	}
	runIn(t, gen, exec.Command("go", "vet", "./..."))
	compileCpp(t, gen)
	tc := tsc.Find(t)
	files := filesUnder(t, gen, e2eTSSuffix)
	for _, c := range tc.Compilers {
		args := append([]string{c.Script, e2eTSNoEmit}, tsc.Args(tc.Typings, files...)...)
		runIn(t, gen, exec.Command(tc.Node, args...))
	}
}

// storedJSON is every json file under gen, in path order, each after a line naming it.
func storedJSON(t *testing.T, gen string) []byte {
	t.Helper()
	var buf bytes.Buffer
	for _, path := range filesUnder(t, gen, e2eJSONSuffix) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		rel, err := filepath.Rel(gen, path)
		if err != nil {
			t.Fatal(err)
		}
		buf.WriteString("== " + filepath.ToSlash(rel) + "\n")
		buf.Write(data)
	}
	return buf.Bytes()
}

// filesUnder is every file under root whose name ends in suffix, sorted.
func filesUnder(t *testing.T, root, suffix string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, suffix) {
			out = append(out, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}
