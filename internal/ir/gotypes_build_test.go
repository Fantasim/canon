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
)

const (
	e2eGoTypesProject     = "testdata/e2e/gotypes.txtar"
	e2eOtherRoot          = "other"
	e2eOtherModule        = "example.com/other"
	e2eGoTypesFile        = "go/a/a.gen.go"
	e2ePredeclaredProject = "testdata/e2e/gopredeclared.txtar"
)

// TestGoTypesModeBuild is CODEGEN.md §5.13 end to end (EVALUATION.md §1, decision 37): a project of go types-mode emits that stage E passes builds through internal/build with no finding; the files package a's @text fns render (§2.9, WIRE.md §8.5) decode back through its public decoders, every source-wire rule and failure kind is checked by gen/go/a/roundtrip_test.go, and both copies of each emit vet.
func TestGoTypesModeBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	dir := t.TempDir()
	writeProject(t, dir, e2eGoTypesProject)
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
	gen, other := filepath.Join(dir, e2eGenRoot), filepath.Join(dir, e2eOtherRoot)
	writeGoMod(t, gen, e2eGoModule)
	writeGoMod(t, other, e2eOtherModule)
	runIn(t, gen, exec.Command("go", "vet", "./..."))
	runIn(t, gen, exec.Command("go", "test", "-race", "-count=1", "-bench=.", "-benchtime=1x", "./..."))
	runIn(t, other, exec.Command("go", "vet", "./..."))
	checkGoTypesCopies(t, gen, other)
}

// TestGoPredeclaredImports is CODEGEN.md §3.4, §2.8 and §5.14: a package Go-named like a predeclared identifier is imported under the alias `<name>pkg` in every mode, so the importing file still calls the builtin, a field `max` stays `max_` and a field `maxpkg` is escaped like any name of an import (testdata/e2e/gopredeclared.txtar).
func TestGoPredeclaredImports(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	dir := t.TempDir()
	writeProject(t, dir, e2ePredeclaredProject)
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
	gen := filepath.Join(dir, e2eGenRoot)
	writeGoMod(t, gen, e2eGoModule)
	runIn(t, gen, exec.Command("go", "vet", "./..."))
	runIn(t, gen, exec.Command("go", "test", "-count=1", "./..."))
	maxpkg := `maxpkg "` + e2eGoModule + `/go/max"`
	for file, alias := range map[string]string{"go/a/a.gen.go": maxpkg, "go/b/b.gen.go": maxpkg, "go/d/d.gen.go": `lenpkg "` + e2eGoModule + `/go/len"`} { //canon:unordered each file alone
		src, err := os.ReadFile(filepath.Join(gen, filepath.FromSlash(file)))
		if err != nil || !strings.Contains(string(src), alias) {
			t.Errorf("%s: no import %s (%v)", file, alias, err)
		}
	}
}

// checkGoTypesCopies is CODEGEN.md §2.1 and §2.8: each copy imports its own rt and the copy of package b under its own root, its go_module's path; and package a reads b's Point with its own reader, never b's public decoder.
func checkGoTypesCopies(t *testing.T, gen, other string) {
	t.Helper()
	for root, module := range map[string]string{gen: e2eGoModule, other: e2eOtherModule} { //canon:unordered each copy alone
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(e2eGoTypesFile)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		for _, want := range []string{`"` + module + `/go/a/rt"`, `"` + module + `/go/b"`, "func decode_b_Point("} {
			if !strings.Contains(text, want) {
				t.Errorf("%s: no %s", module, want)
			}
		}
		if strings.Contains(text, "b.DecodePoint(") {
			t.Errorf("%s: package a calls b's public decoder", module)
		}
	}
}

// writeGoMod writes dir/go.mod for module, at the installed Go's language version.
func writeGoMod(t *testing.T, dir, module string) {
	t.Helper()
	mod := "module " + module + "\n\ngo " + strings.TrimPrefix(version.Lang(runtime.Version()), "go") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), e2eFileMode); err != nil {
		t.Fatal(err)
	}
}
