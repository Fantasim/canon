package tsgen_test

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/testkit/tsc"
)

const (
	expectedDir = "expected"
	tsSuffix    = ".ts"
	testSuffix  = ".test.js"
	timeout     = 5 * time.Minute
)

// run runs node with args under the toolchain's timeout, in dir when given, and returns its combined output.
func run(tc tsc.Toolchain, args []string, dir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, tc.Node, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// typecheck type-checks files with every compiler, `--strict --noEmit` (CODEGEN.md §9).
func typecheck(t *testing.T, tc tsc.Toolchain, files ...string) {
	t.Helper()
	for _, c := range tc.Compilers {
		args := append([]string{c.Script, "--noEmit"}, tsc.Args(tc.Typings, files...)...)
		if out, err := run(tc, args, ""); err != nil {
			t.Errorf("%s: %v\n%s", c.Name, err, out)
		}
	}
}

// compile builds files, rooted at root, with the first compiler (the 5.0 floor) into a temporary ES module directory.
func compile(t *testing.T, tc tsc.Toolchain, root string, files ...string) string {
	t.Helper()
	out := t.TempDir()
	args := append([]string{tc.Compilers[0].Script, "--outDir", out, "--rootDir", root}, tsc.Args(tc.Typings, files...)...)
	if res, err := run(tc, args, ""); err != nil {
		t.Fatalf("%s: %v\n%s", tc.Compilers[0].Name, err, res)
	}
	if err := os.WriteFile(filepath.Join(out, "package.json"), []byte("{\"type\":\"module\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return out
}

// runTests runs every `*.test.js` under out with `node --test`.
func runTests(t *testing.T, tc tsc.Toolchain, out string) {
	t.Helper()
	var tests []string
	err := filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, testSuffix) {
			tests = append(tests, p)
		}
		return err
	})
	if err != nil || len(tests) == 0 {
		t.Fatalf("no test was compiled: %v", err)
	}
	if res, err := run(tc, append([]string{"--test"}, tests...), out); err != nil {
		t.Errorf("node --test: %v\n%s", err, res)
	}
}

// goldenTS are the generated TypeScript files the examples' goldens hold, in path order.
func goldenTS(t *testing.T) (root string, files []string) {
	t.Helper()
	root, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, tsSuffix) && strings.Contains(filepath.ToSlash(p), "/"+expectedDir+"/") {
			files = append(files, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return root, files
}

// CODEGEN.md §9, IMPLEMENTATION-PLAN.md M6: every example golden passes `tsc --strict --noEmit` (5.0 and current), with no unused local (§8.2).
func TestGoldensTypecheck(t *testing.T) {
	tc := tsc.Find(t)
	_, files := goldenTS(t)
	if len(files) == 0 {
		t.Fatal("no TypeScript golden")
	}
	typecheck(t, tc, files...)
}

// CONFORMANCE.md §7.1: the conformance tests of the goldens run under `node --test` against the generated code, with the drivers of testdata/drivers-examples beside them.
func TestGoldensConformance(t *testing.T) {
	tc := tsc.Find(t)
	root, files := goldenTS(t)
	out := compile(t, tc, root, files...)
	drivers, err := filepath.Glob(filepath.Join("testdata", "drivers-examples", "*"+testSuffix))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drivers {
		body, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(out, filepath.Base(d)), body)
	}
	for _, rel := range []string{"features/ts/expected/ts/out/skills.json", "features/copies/expected/copies/out/data/badges.json"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(out, filepath.FromSlash(rel)), data)
	}
	runTests(t, tc, out)
}

// writeOutputs writes a world's generated files under dir, at their project-relative paths, and returns them.
func writeOutputs(t *testing.T, dir string, outs []output) []string {
	t.Helper()
	var paths []string
	for _, o := range outs {
		p := filepath.Join(dir, filepath.FromSlash(o.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, o.Content, 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return paths
}

// writeCases generates every case of testdata/ts into dir, one directory per case, and returns the files.
func writeCases(t *testing.T, dir string) []string {
	t.Helper()
	cases, err := golden.Load("testdata/ts/*.txtar", golden.Expected(generatedTS))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, c := range cases {
		w := buildWorld(t, c.Archive.Files)
		name := strings.TrimSuffix(filepath.Base(c.Path), filepath.Ext(c.Path))
		paths = append(paths, writeOutputs(t, filepath.Join(dir, name), w.generate(t))...)
	}
	return paths
}

// CODEGEN.md §8, §9: every generated case, baked, data or types, passes `tsc --strict` (5.0 and current).
func TestCasesTypecheck(t *testing.T) {
	tc := tsc.Find(t)
	dir := t.TempDir()
	typecheck(t, tc, writeCases(t, dir)...)
}

// CONFORMANCE.md §7: the conformance tests of every case run under `node --test`.
func TestCasesConformance(t *testing.T) {
	tc := tsc.Find(t)
	dir := t.TempDir()
	runTests(t, tc, compile(t, tc, dir, writeCases(t, dir)...))
}
