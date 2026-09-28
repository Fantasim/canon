package ir_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	e2eTypesProject = "testdata/e2e/types.txtar"
	e2eRefused      = "testdata/e2e/refused/*.txtar"
)

// TestTypesModeBuild is CODEGEN.md §5.13 end to end (EVALUATION.md §1, decision 37): a project of cpp types-mode emits that stage E passes builds through internal/build with no finding, and its C++ compiles, so gen/cpp refuses nothing stage E let through.
func TestTypesModeBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	dir := t.TempDir()
	writeProject(t, dir, e2eTypesProject)
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
	compileCpp(t, filepath.Join(dir, e2eGenRoot))
}

// TestTypesModeRefusals is CODEGEN.md §5.13's refusals through internal/build, whose folder, unlike the ir test world's, folds a record default: each archive checks to its findings.txt.
func TestTypesModeRefusals(t *testing.T) {
	golden.Run(t, e2eRefused, func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		dir := t.TempDir()
		for _, f := range c.Archive.Files {
			path := filepath.Join(dir, filepath.FromSlash(f.Name))
			if err := os.MkdirAll(filepath.Dir(path), e2eDirMode); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, f.Data, e2eFileMode); err != nil {
				t.Fatal(err)
			}
		}
		p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Check(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		return []byte(renderFindings(t, res.Findings))
	}, golden.Expected(findingsFile))
}

// renderFindings is the golden text form of a build's or check's findings.
func renderFindings(t *testing.T, f build.Findings) string {
	t.Helper()
	var buf bytes.Buffer
	if err := diag.Render(&buf, f.Files, f.List, diag.RenderOptions{Summary: f.Summary, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
