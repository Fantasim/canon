package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

const (
	examplesDir = "../../../examples"
	viewsOwner  = "views"
)

// VIEWMODEL.md §16: no example has a view finding, every root redirected.
func TestExamplesHaveNoViewFinding(t *testing.T) {
	dir, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	roots := map[string]string{"resource": "_fixtures/resource", "client": "_fixtures/client"}
	for _, name := range []string{"source", "services", "sovcommon", "web", "parity", "generated"} {
		roots[name] = filepath.ToSlash(filepath.Join(out, name))
		if err := os.MkdirAll(filepath.Join(out, name), 0o750); err != nil { // SPEC §3.1: a required root exists
			t.Fatal(err)
		}
	}
	x := analyze(t, project.OS(), filepath.ToSlash(dir), build.Options{Roots: roots})
	if len(x.bags) == 0 {
		t.Fatal("no package analyzed")
	}
	all, _ := x.findings()
	for _, f := range all {
		if owner(f.Code) == viewsOwner {
			t.Errorf("%s: %s %s", f.Package, f.Code, f.Message)
		}
	}
}

// owner is the package ERRORS.md gives a code.
func owner(code diag.Code) string {
	for _, d := range diag.Registry {
		if d.Code == code {
			return d.Package
		}
	}
	return ""
}
