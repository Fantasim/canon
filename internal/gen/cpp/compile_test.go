package cppgen_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// writeFiles writes the generated files into dir.
func writeFiles(t *testing.T, dir string, files []ir.File) {
	t.Helper()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.Path), f.Content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// writeTree writes the generated files and the driver, as main.cpp, into dir.
func writeTree(t *testing.T, dir string, files []ir.File, driver string) {
	t.Helper()
	writeFiles(t, dir, files)
	copyFile(t, filepath.Join("testdata", "main", driver), filepath.Join(dir, "main.cpp"), same)
}

// copyFile copies a committed data file into the test tree.
func copyFile(t *testing.T, from, to string, edit func([]byte) []byte) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, edit(b), 0o644); err != nil {
		t.Fatal(err)
	}
}

func same(b []byte) []byte { return b }

// CPP-06, CODEGEN.md §5.9, §5.11, §9, IMPLEMENTATION-PLAN.md §6 M2 items 2 and 5.
func TestPipelineCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, pipeline()), "pipeline_main.cpp")
	data := filepath.Join("..", "..", "..", "examples", "pipeline", "expected", "potions.json")
	copyFile(t, data, filepath.Join(dir, "good", "potions.json"), same)
	copyFile(t, data, filepath.Join(dir, "stale", "potions.json"), func(b []byte) []byte {
		return bytes.Replace(b, []byte(pipelineSchema), []byte("pipeline.Potion@00000000"), 1)
	})
	if err := os.MkdirAll(filepath.Join(dir, "missing"), 0o755); err != nil {
		t.Fatal(err)
	}
	outs := buildAndRun(t, dir, []string{"pipeline.gen.cpp", "pipeline_conformance.gen.cpp", "main.cpp"}, dir)
	for _, out := range outs {
		if !strings.Contains(out, "failures: 0\n") {
			t.Errorf("driver output:\n%s", out)
		}
		want := "stale: " + filepath.Join(dir, "stale", "potions.json") + ": built from schema pipeline.Potion@00000000, " +
			"this binary expects pipeline.Potion@f750790e. Rebuild the data or deploy the matching binary\n"
		if !strings.Contains(out, want) {
			t.Errorf("schema refusal:\n%s\nwant the line\n%s", out, want)
		}
	}
}

// CONFORMANCE.md §7.2: a disagreeing vector is counted and printed in the §7.2 format.
func TestConformanceReportsAFailure(t *testing.T) {
	t.Parallel()
	p := pipeline()
	healFor := p.Types[0].(*ir.Record).Methods[1]
	healFor.Vectors[0].Want = num(201)
	healFor.Vectors[3].Code = codeOverflow
	dir := t.TempDir()
	writeTree(t, dir, generate(t, p), "conformance_main.cpp")
	compilers, include := cxx.Toolchain(t)
	for _, cc := range compilers {
		bin := filepath.Join(dir, filepath.Base(cc))
		args := append(append([]string(nil), cxx.Flags...), "-I", include, "-o", bin,
			filepath.Join(dir, "pipeline_conformance.gen.cpp"), filepath.Join(dir, "main.cpp"))
		if out, err := compile(cc, args, dir); err != nil {
			t.Fatalf("%s: %v\n%s", cc, err, out)
		}
		var stdout, stderr bytes.Buffer
		run := exec.Command(bin)
		run.Stdout, run.Stderr = &stdout, &stderr
		if err := run.Run(); err != nil {
			t.Fatal(err)
		}
		if textOut(stdout.String()) != "failures: 2\n" {
			t.Errorf("%s: %q", cc, stdout.String())
		}
		want := "pipeline: Potion.healFor(heal=500, missingHp=200) = 200 [], canon says 201 []\n" +
			"pipeline: Potion.healFor(heal=500, missingHp=-9223372036854775808) = 0 [], canon says 0 [" + string(codeOverflow) + "]\n"
		if textOut(stderr.String()) != want {
			t.Errorf("%s: stderr\n%s\nwant\n%s", cc, stderr.String(), want)
		}
	}
}

// CODEGEN.md §4, §5, §7.2, §7.6, CONFORMANCE.md §2–§3: every construct compiles and reads back.
func TestConstructsCompileAndRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := generate(t, constructs())
	writeTree(t, dir, files, "shop_main.cpp")
	for _, sub := range []string{"good", "bad", "badsnap"} {
		entries, err := os.ReadDir(filepath.Join("testdata", "main", "shop", sub))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			copyFile(t, filepath.Join("testdata", "main", "shop", sub, e.Name()), filepath.Join(dir, sub, e.Name()), same)
		}
	}
	outs := buildAndRun(t, dir, []string{"shop.gen.cpp", "shop_conformance.gen.cpp", "main.cpp"}, dir)
	for _, out := range outs {
		if !strings.Contains(out, "failures: 0\n") {
			t.Errorf("driver output:\n%s", out)
		}
		want := "bad: " + filepath.Join(dir, "bad", "shelves.json") + ": rows[1].slots: expected an integer from -128 to 127\n"
		if !strings.Contains(out, want) {
			t.Errorf("decode error:\n%s\nwant the line\n%s", out, want)
		}
		want = "badsnap: " + filepath.Join(dir, "badsnap") + "/items.json: rows[2].$best: no entry nope\n"
		if !strings.Contains(out, want) {
			t.Errorf("resolve error:\n%s\nwant the line\n%s", out, want)
		}
	}
}

// A lookup argument that is no member of its enum aborts instead of reading past the table
// (log-2026-09-24, gen/cpp review calls).
func TestLookupNonMemberAborts(t *testing.T) {
	t.Parallel()
	compilers, include := cxx.Toolchain(t)
	dir := t.TempDir()
	writeTree(t, dir, generate(t, constructs()), "abort_main.cpp")
	bin := filepath.Join(dir, "abort")
	args := append(append([]string(nil), cxx.Flags...), "-I", dir, "-I", include, "-o", bin, filepath.Join(dir, "main.cpp"))
	if out, err := compile(compilers[0], args, dir); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	err := exec.Command(bin).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.Exited() {
		t.Errorf("run: %v, want an abort", err)
	}
}
