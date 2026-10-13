package golden

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/tsc"
)

const (
	membershipProject = "testdata/membership/keyed.txtar"
	refusedProject    = "testdata/membership/refused.txtar"
	goModFile         = "go.mod"
	goModText         = "module example.com/gen\n\ngo 1.25\n"
	genDir            = "gen"
	cppSources        = "demo.gen.cpp"
	cppMain           = "main.cpp"
	tsMain            = "run.ts"
	tsMainJS          = "run.js"
	tsModuleType      = "package.json"
	tsModuleText      = "{\"type\":\"module\"}\n"
	runDir            = "run"
	goMain            = "main.go"
	nodeCommand       = "node"
	answerLines       = 10
	answerPath        = "answers:lines.l%02d.text"
	refusedFile       = "demo/demo.canon"

	goRunner = `package main

import (
	"fmt"

	demo "example.com/gen/go"
)

func main() {
	fns := []struct {
		name string
		got  bool
	}{{"spawnRef", demo.SpawnRef()}, {"firstSpawn", demo.FirstSpawn()}, {"wolfKey", demo.WolfKey()}, {"pooledRef", demo.PooledRef()}, {"pooledKey", demo.PooledKey()}}
	for _, f := range fns {
		fmt.Printf("fn %s %v\n", f.name, f.got)
	}
	if v, ok := demo.IndexOfRef(); ok {
		fmt.Printf("fn indexOfRef %d\n", v)
	} else {
		fmt.Println("fn indexOfRef none")
	}
	for i := 0; i < demo.GetPool().Len(); i++ {
		p := demo.GetPool().At(i)
		fmt.Printf("row pool %d %v %v %v\n", i, p.RefIn(), p.SelfIn(), p.Keyed())
	}
	p := demo.GetKs().At(0)
	fmt.Printf("row ks 0 %v %v %v\n", p.RefIn(), p.SelfIn(), p.Keyed())
}
`

	cppRunner = `#include <cstdio>
#include "demo.gen.h"

static const char* B(bool v) { return v ? "true" : "false"; }

static void Row(const char* table, size_t i, const demo::P& p) {
    std::printf("row %s %zu %s %s %s\n", table, i, B(p.RefIn()), B(p.SelfIn()), B(p.Keyed()));
}

int main() {
    std::printf("fn spawnRef %s\nfn firstSpawn %s\nfn wolfKey %s\nfn pooledRef %s\nfn pooledKey %s\n", B(demo::SpawnRef()),
                B(demo::FirstSpawn()), B(demo::WolfKey()), B(demo::PooledRef()), B(demo::PooledKey()));
    const auto index = demo::IndexOfRef();
    if (index) {
        std::printf("fn indexOfRef %lld\n", static_cast<long long>(*index));
    } else {
        std::printf("fn indexOfRef none\n");
    }
    for (size_t i = 0; i < demo::GetPool().Len(); i++) {
        Row("pool", i, demo::GetPool().At(i));
    }
    Row("ks", 0, demo::GetKs().At(0));
    return 0;
}
`

	tsRunner = `import { spawnRef, firstSpawn, wolfKey, pooledRef, pooledKey, indexOfRef, pool, ks } from "./demo.js";

const lines: string[] = [
  "fn spawnRef " + spawnRef(), "fn firstSpawn " + firstSpawn(), "fn wolfKey " + wolfKey(),
  "fn pooledRef " + pooledRef(), "fn pooledKey " + pooledKey(), "fn indexOfRef " + (indexOfRef() ?? "none"),
];
pool.all.forEach((p, i) => lines.push("row pool " + i + " " + p.refIn + " " + p.selfIn + " " + p.keyed));
lines.push("row ks 0 " + ks.at(0).refIn + " " + ks.at(0).selfIn + " " + ks.at(0).keyed);
console.log(lines.join("\n"));
`
)

// openProject writes the archive at path under a temporary directory and opens it.
func openProject(t *testing.T, path string) (string, *canon.Project) {
	t.Helper()
	cases, err := Load(path)
	if err != nil || len(cases) != 1 {
		t.Fatalf("load %s: %v", path, err)
	}
	dir := t.TempDir()
	for _, f := range cases[0].Archive.Files {
		writeTestFile(t, filepath.Join(dir, filepath.FromSlash(f.Name)), f.Data)
	}
	p, err := canon.Open(dir, canon.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return dir, p
}

// writeTestFile writes data at path, creating its directory.
func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, filePerm); err != nil {
		t.Fatal(err)
	}
}

// buildProject builds the targets of the project in dir and returns its generated tree; an error finding fails the test.
func buildProject(t *testing.T, p *canon.Project, dir string, targets ...canon.Target) string {
	t.Helper()
	res, err := p.Build(context.Background(), canon.BuildOptions{Targets: targets})
	if err != nil || res.Check.Summary.Errors > 0 {
		t.Fatalf("build: %v, %+v", err, res.Check.Findings)
	}
	return filepath.Join(dir, genDir)
}

// run runs name with args in dir and returns its stdout; a failure fails the test with the output.
func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

// STDLIB.md §5, DECISIONS 314/317, log-2026-10-05 showcase #1: the evaluator's own tests of keyed.txtar pass, so its expected answers are the evaluator's.
func TestMembershipEvaluatorIsTheOracle(t *testing.T) {
	_, p := openProject(t, membershipProject)
	res, err := p.Test(context.Background(), canon.TestOptions{})
	if err != nil || res.Check.Summary.Errors > 0 {
		t.Fatalf("test: %v, %+v", err, res.Check.Findings)
	}
	if res.Failed > 0 || res.Passed == 0 {
		t.Fatalf("evaluator tests: %d passed, %d failed: %+v", res.Passed, res.Failed, res.Tests)
	}
}

// STDLIB.md §5, DECISIONS 314/317 (log-2026-10-05 showcase #1, F9): the generated Go returns what the evaluator answers for `ref in keyedList`, `contains`, `indexOf` and `hasKey`, in package fns and in a method of every row.
func TestMembershipGoAgrees(t *testing.T) {
	dir, p := openProject(t, membershipProject)
	want := evaluatorLines(t, p)
	gen := buildProject(t, p, dir, canon.TargetGo)
	writeTestFile(t, filepath.Join(gen, goModFile), []byte(goModText))
	writeTestFile(t, filepath.Join(gen, runDir, goMain), []byte(goRunner))
	if got := run(t, gen, "go", "run", "./"+runDir); got != want {
		t.Errorf("go prints\n%s\nwant\n%s", got, want)
	}
}

// STDLIB.md §5, DECISIONS 314/317 (F9): the generated C++ returns what the evaluator answers, with every compiler and mode.
func TestMembershipCppAgrees(t *testing.T) {
	dir, p := openProject(t, membershipProject)
	want := evaluatorLines(t, p)
	gen := filepath.Join(buildProject(t, p, dir, canon.TargetCpp), "cpp")
	writeTestFile(t, filepath.Join(gen, cppMain), []byte(cppRunner))
	for _, got := range buildAndRun(t, gen, []string{cppSources, cppMain}) {
		if got != want {
			t.Errorf("c++ prints\n%s\nwant\n%s", got, want)
		}
	}
}

// STDLIB.md §5, DECISIONS 314/317 (F9): the generated TypeScript returns what the evaluator answers, under both compilers.
func TestMembershipTypeScriptAgrees(t *testing.T) {
	tc := tsc.Find(t)
	dir, p := openProject(t, membershipProject)
	want := evaluatorLines(t, p)
	gen := filepath.Join(buildProject(t, p, dir, canon.TargetTS), "ts")
	writeTestFile(t, filepath.Join(gen, tsMain), []byte(tsRunner))
	for _, c := range tc.Compilers {
		out := t.TempDir()
		args := append([]string{c.Script, "--outDir", out, "--rootDir", gen},
			tsc.Args(tc.Typings, filepath.Join(gen, "demo.ts"), filepath.Join(gen, tsMain))...)
		run(t, gen, tc.Node, args...)
		writeTestFile(t, filepath.Join(out, tsModuleType), []byte(tsModuleText))
		if got := run(t, out, nodeCommand, tsMainJS); got != want {
			t.Errorf("%s: typescript prints\n%s\nwant\n%s", c.Name, got, want)
		}
	}
}

// evaluatorLines is what every generated runtime must print: the lines package answers evaluates, in order, one per entry of its lines table (keyed.txtar).
func evaluatorLines(t *testing.T, p *canon.Project) string {
	t.Helper()
	var out strings.Builder
	for i := 0; i < answerLines; i++ {
		v, err := p.Value(context.Background(), fmt.Sprintf(answerPath, i))
		if err != nil {
			t.Fatal(err)
		}
		out.WriteString(v.String() + "\n")
	}
	return out.String()
}

// fnLine is the 1-based line of the first line of file holding decl.
func fnLine(t *testing.T, file, decl string) int {
	t.Helper()
	for i, line := range strings.Split(file, "\n") {
		if strings.Contains(line, decl) {
			return i + 1
		}
	}
	t.Fatalf("no %s in the fixture", decl)
	return 0
}

// CONFORMANCE.md §2.2 (E9001), STDLIB.md §5: `ref in keyedList` and `match` are outside the portable subset, so no target translates them: one E9001 inside each of the three fns, in order.
func TestMembershipIsNeverTranslated(t *testing.T) {
	dir, p := openProject(t, refusedProject)
	res, err := p.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(dir, refusedFile))
	if err != nil {
		t.Fatal(err)
	}
	starts := []int{fnLine(t, string(src), "fn probe("), fnLine(t, string(src), "fn byMatch("), fnLine(t, string(src), "fn weighted("), len(strings.Split(string(src), "\n")) + 1}
	if len(res.Findings) != len(starts)-1 {
		t.Fatalf("findings %+v, want one per fn", res.Findings)
	}
	for i, f := range res.Findings {
		if f.Code != string(diag.E9001.Def().Code) || f.Line < starts[i] || f.Line >= starts[i+1] {
			t.Errorf("finding %d is %s at line %d, want %s inside the fn at line %d", i, f.Code, f.Line, diag.E9001.Def().Code, starts[i])
		}
	}
}
