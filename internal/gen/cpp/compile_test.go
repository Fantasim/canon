package cppgen_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/ir"
)

// The toolchains and flags of IMPLEMENTATION-PLAN.md §7.8, CODEGEN.md §9 and CONFORMANCE.md §5.
var (
	cxxCompilers = []string{"g++", "clang++"}
	cxxFlags     = []string{"-std=c++17", "-Wall", "-Wextra", "-Wpedantic", "-Werror", "-ffp-contract=off", "-O1"}
	cxxModes     = [][]string{nil, {"-fno-exceptions"}}
	// nlohmannDirs: the Sovereign checkout's vendored copy (read only), or a system install.
	nlohmannDirs = []string{filepath.Join("..", "..", "..", "..", "..", "Source", "External"), "/usr/include"}
)

const compileTimeout = 5 * time.Minute

// toolchain finds the compilers and the nlohmann/json include directory, or skips.
func toolchain(t *testing.T) (compilers []string, include string) {
	t.Helper()
	for _, c := range cxxCompilers {
		if p, err := exec.LookPath(c); err == nil {
			compilers = append(compilers, p)
		}
	}
	if len(compilers) == 0 {
		t.Skip("no C++ compiler (g++, clang++) on PATH")
	}
	for _, d := range nlohmannDirs {
		if _, err := os.Stat(filepath.Join(d, "nlohmann", "json.hpp")); err == nil {
			abs, err := filepath.Abs(d)
			if err != nil {
				t.Fatal(err)
			}
			return compilers, abs
		}
	}
	t.Skip("nlohmann/json.hpp not found")
	return nil, ""
}

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

// buildAndRun compiles sources in dir with every toolchain and mode, runs each binary with
// args, and returns the output of each run; a failed build or a non-zero exit fails the test.
func buildAndRun(t *testing.T, dir string, sources []string, args ...string) []string {
	t.Helper()
	compilers, include := toolchain(t)
	var outs []string
	for _, cxx := range compilers {
		for i, mode := range cxxModes {
			bin := filepath.Join(dir, filepath.Base(cxx)+"-"+string(rune('a'+i)))
			cmdArgs := append(append(append([]string(nil), cxxFlags...), mode...), "-I", dir, "-I", include, "-o", bin)
			for _, s := range sources {
				cmdArgs = append(cmdArgs, filepath.Join(dir, s))
			}
			ctx, cancel := context.WithTimeout(context.Background(), compileTimeout)
			out, err := exec.CommandContext(ctx, cxx, cmdArgs...).CombinedOutput()
			cancel()
			if err != nil {
				t.Fatalf("%s %s: %v\n%s", filepath.Base(cxx), strings.Join(mode, " "), err, out)
			}
			var stdout, stderr bytes.Buffer
			run := exec.Command(bin, args...)
			run.Stdout, run.Stderr = &stdout, &stderr
			if err := run.Run(); err != nil {
				t.Fatalf("%s %s: run: %v\n%s%s", filepath.Base(cxx), strings.Join(mode, " "), err, stdout.String(), stderr.String())
			}
			outs = append(outs, stdout.String())
		}
	}
	return outs
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
	p := pipeline()
	healFor := p.Types[0].(*ir.Record).Methods[1]
	healFor.Vectors[0].Want = num(201)
	healFor.Vectors[3].Code = codeOverflow
	dir := t.TempDir()
	writeTree(t, dir, generate(t, p), "conformance_main.cpp")
	compilers, include := toolchain(t)
	for _, cxx := range compilers {
		bin := filepath.Join(dir, filepath.Base(cxx))
		args := append(append([]string(nil), cxxFlags...), "-I", include, "-o", bin,
			filepath.Join(dir, "pipeline_conformance.gen.cpp"), filepath.Join(dir, "main.cpp"))
		if out, err := exec.Command(cxx, args...).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", cxx, err, out)
		}
		var stdout, stderr bytes.Buffer
		run := exec.Command(bin)
		run.Stdout, run.Stderr = &stdout, &stderr
		if err := run.Run(); err != nil {
			t.Fatal(err)
		}
		if stdout.String() != "failures: 2\n" {
			t.Errorf("%s: %q", cxx, stdout.String())
		}
		want := "pipeline: Potion.healFor(heal=500, missingHp=200) = 200 [], canon says 201 []\n" +
			"pipeline: Potion.healFor(heal=500, missingHp=-9223372036854775808) = 0 [], canon says 0 [" + string(codeOverflow) + "]\n"
		if stderr.String() != want {
			t.Errorf("%s: stderr\n%s\nwant\n%s", cxx, stderr.String(), want)
		}
	}
}

// CODEGEN.md §4, §5, §7.2, §7.6, CONFORMANCE.md §2–§3: every construct compiles and reads back.
func TestConstructsCompileAndRun(t *testing.T) {
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
		want := "bad: " + filepath.Join(dir, "bad", "shelves.json") + ": rows[1].slots: expected an integer\n"
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
	compilers, include := toolchain(t)
	dir := t.TempDir()
	writeTree(t, dir, generate(t, constructs()), "abort_main.cpp")
	bin := filepath.Join(dir, "abort")
	args := append(append([]string(nil), cxxFlags...), "-I", dir, "-I", include, "-o", bin, filepath.Join(dir, "main.cpp"))
	if out, err := exec.Command(compilers[0], args...).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	err := exec.Command(bin).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.Exited() {
		t.Errorf("run: %v, want an abort", err)
	}
}
