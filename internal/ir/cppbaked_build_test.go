package ir_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// e2eCppBaked are the probes of a baked cpp emit, one construct each.
const e2eCppBaked = "testdata/e2e/cppbaked/*.txtar"

// TestCppBakedProbes is CODEGEN.md §2.1 and decision 37 for a baked cpp emit: `canon check` and `canon build` report the same findings, the build never fails where check passed, and whatever it writes compiles; findings.txt is the build's findings.
func TestCppBakedProbes(t *testing.T) {
	golden.Run(t, e2eCppBaked, func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		dir := t.TempDir()
		writeProject(t, dir, c.Path)
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
			compileCpp(t, filepath.Join(dir, e2eGenRoot))
		}
		return []byte(out)
	}, golden.Expected(findingsFile))
}
