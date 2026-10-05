package ir_test

import (
	"context"
	"go/version"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/testkit/tsc"
)

// e2eShared are the Emberfall shapes, one mode of the holding package each; e2eLifted the stage-E refusals DECISIONS 320 and 323 lifted, each a small program of its own, whose `@features` outputs go under gen.
const (
	e2eShared   = "testdata/e2e/shared/*.txtar"
	e2eLifted   = "testdata/e2e/lifted/*.txtar"
	e2eProjFile = "project.canon"
	e2eGoSuffix = ".go"
	// e2eFeatures is the project a lifted case runs in: its `@features` root is gen, a go_module root.
	e2eFeatures = "project demo {\n  canon: \"0.1\"\n  roots {\n    features: \"gen\"\n  }\n  go_module {\n    features: \"example.com/gen\"\n  }\n}\n"
)

// TestSharedRecordsBuild is DECISIONS 320 and 323 end to end (CODEGEN.md §2.2, §2.8, §5.14): a package holds another package's records, variants, cases, dependent types and tables in every target and in every mode its generators build; `canon check` and `canon build` report the same findings, the build never fails where check passed, and a clean build's Go vets, its C++ and its TypeScript compile; findings.txt is the build's findings.
func TestSharedRecordsBuild(t *testing.T) {
	for _, glob := range []string{e2eShared, e2eLifted} {
		golden.Run(t, glob, func(t *testing.T, c golden.Case) []byte {
			t.Helper()
			return sharedBuild(t, c.Path)
		}, golden.Expected(findingsFile))
	}
}

// sharedBuild checks and builds the project of the archive at path, a lifted case in e2eFeatures, and returns the build's findings.
func sharedBuild(t *testing.T, path string) []byte {
	t.Helper()
	dir := t.TempDir()
	writeProject(t, dir, path)
	if _, err := os.Stat(filepath.Join(dir, e2eProjFile)); err != nil {
		if err := os.WriteFile(filepath.Join(dir, e2eProjFile), []byte(e2eFeatures), e2eFileMode); err != nil {
			t.Fatal(err)
		}
	}
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	checked, err := p.Check(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{})
	if err != nil {
		t.Fatalf("check passed with %d errors, then build failed: %v", checked.Summary.Errors, err)
	}
	out := renderFindings(t, res.Findings)
	if want := renderFindings(t, checked.Findings); out != want {
		t.Errorf("build findings differ from check's:\n%s\nwant\n%s", out, want)
	}
	if res.Summary.Errors == 0 && !testing.Short() {
		compileWritten(t, filepath.Join(dir, e2eGenRoot))
	}
	return []byte(out)
}

// compileWritten vets the Go, and compiles the C++ and the TypeScript, a build wrote under gen, each target it wrote.
func compileWritten(t *testing.T, gen string) {
	t.Helper()
	if len(filesUnder(t, gen, e2eGoSuffix)) > 0 {
		mod := e2eGoModPrefix + e2eGoModule + e2eGoModGo + strings.TrimPrefix(version.Lang(runtime.Version()), "go") + "\n"
		if err := os.WriteFile(filepath.Join(gen, e2eGoModFile), []byte(mod), e2eFileMode); err != nil {
			t.Fatal(err)
		}
		runIn(t, gen, exec.Command("go", "vet", "./..."))
	}
	if len(filesUnder(t, gen, e2eCppSuffix)) > 0 {
		compileCpp(t, gen)
	}
	files := filesUnder(t, gen, e2eTSSuffix)
	if len(files) == 0 {
		return
	}
	tc := tsc.Find(t)
	for _, c := range tc.Compilers {
		args := append([]string{c.Script, e2eTSNoEmit}, tsc.Args(tc.Typings, files...)...)
		runIn(t, gen, exec.Command(tc.Node, args...))
	}
}
