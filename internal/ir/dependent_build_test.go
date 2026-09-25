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
	"github.com/fantasim/canonlang/internal/testkit/cxx"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	e2eProject   = "testdata/e2e/dependent.txtar"
	e2eGenRoot   = "gen"
	e2eGoModule  = "example.com/gen"
	e2eCppSuffix = ".gen.cpp"
	e2eDirMode   = 0o750
	e2eFileMode  = 0o600
)

// TestDependentTypesBuild is CODEGEN.md §5.6 and §2.8 end to end (EVALUATION.md §1, decision 37): a project whose dependent types stage E passes builds through internal/build with no finding, its Go outputs (data and baked) vet and its C++ data-mode outputs compile, so no generator refuses what stage E let through.
func TestDependentTypesBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	dir := t.TempDir()
	writeProject(t, dir)
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Errors > 0 {
		var buf bytes.Buffer
		if err := diag.Render(&buf, res.Files, res.List, diag.RenderOptions{Summary: res.Summary, Golden: true}); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("build findings:\n%s", buf.String())
	}
	gen := filepath.Join(dir, e2eGenRoot)
	mod := "module " + e2eGoModule + "\n\ngo " + strings.TrimPrefix(version.Lang(runtime.Version()), "go") + "\n"
	if err := os.WriteFile(filepath.Join(gen, "go.mod"), []byte(mod), e2eFileMode); err != nil {
		t.Fatal(err)
	}
	runIn(t, gen, exec.Command("go", "vet", "./..."))
	compileCpp(t, gen)
}

// writeProject writes the archive's files under dir.
func writeProject(t *testing.T, dir string) {
	t.Helper()
	cases, err := golden.Load(e2eProject)
	if err != nil || len(cases) != 1 {
		t.Fatalf("load %s: %v", e2eProject, err)
	}
	for _, f := range cases[0].Archive.Files {
		path := filepath.Join(dir, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(path), e2eDirMode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, f.Data, e2eFileMode); err != nil {
			t.Fatal(err)
		}
	}
}

// compileCpp checks every generated .gen.cpp under root with each compiler found, with the flags every compile uses.
func compileCpp(t *testing.T, root string) {
	t.Helper()
	compilers, include := cxx.Toolchain(t)
	var sources []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, e2eCppSuffix) {
			sources = append(sources, path)
		}
		return err
	})
	if err != nil || len(sources) == 0 {
		t.Fatalf("no C++ sources under %s: %v", root, err)
	}
	slices.Sort(sources)
	for _, cc := range compilers {
		for _, src := range sources {
			args := append(slices.Clone(cxx.Flags), "-fsyntax-only", "-I", include, src)
			runIn(t, root, exec.Command(cc, args...))
		}
	}
}

// runIn runs cmd in dir, failing the test with its output on error.
func runIn(t *testing.T, dir string, cmd *exec.Cmd) {
	t.Helper()
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", strings.Join(cmd.Args, " "), err, out)
	}
}
