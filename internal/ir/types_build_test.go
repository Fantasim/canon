package ir_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

const e2eTypesProject = "testdata/e2e/types.txtar"

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

// renderFindings is the golden text form of a build's or check's findings.
func renderFindings(t *testing.T, f build.Findings) string {
	t.Helper()
	var buf bytes.Buffer
	if err := diag.Render(&buf, f.Files, f.List, diag.RenderOptions{Summary: f.Summary, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
