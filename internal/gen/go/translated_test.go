package gogen_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

const translatedGoldens = "testdata/translated"

// calcFiles generates the calc package twice: equal outputs (CODEGEN.md §2.7).
func calcFiles(t *testing.T, p *ir.Package) map[string][]byte {
	t.Helper()
	first, err := gogen.Generate(p, p.Emits[0])
	if err != nil {
		t.Fatal(err)
	}
	again, err := gogen.Generate(p, p.Emits[0])
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for i, f := range first {
		if !bytes.Equal(f.Content, again[i].Content) {
			t.Errorf("%s differs between two runs", f.Path)
		}
		out[f.Path] = f.Content
	}
	return out
}

// CODEGEN.md §5.10, CONFORMANCE.md §2–§3, §7: translated methods and package fns, and the conformance file, equal their goldens.
func TestTranslatedGolden(t *testing.T) {
	files := calcFiles(t, newCalc().pkg())
	checkGoldens(t, files, sortedPaths(files), translatedGoldens)
	for _, path := range sortedPaths(files) {
		if n := headerLines(path, files[path]); n > 2 {
			t.Errorf("%s: header is %d comment lines, want at most 2 (decision 192)", path, n)
		}
	}
}

// CONFORMANCE.md §1, §7.2: the generated module's go test -race runs every vector and passes.
func TestTranslatedConformanceRuns(t *testing.T) {
	if out, err := goTest(t, calcFiles(t, newCalc().pkg())); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

// CONFORMANCE.md §7.2: a vector the translation disagrees with fails, and the message names the call, what Go got and what canon says, with both codes.
func TestTranslatedConformanceFails(t *testing.T) {
	c := newCalc()
	p := c.pkg()
	add := p.Fns[3]
	if add.Name != "add" {
		t.Fatalf("fn 3 is %s, want add", add.Name)
	}
	add.Vectors[0].Want = &value.Int{V: 4}
	add.Vectors[1].Code = ""
	add.Vectors[1].Want = &value.Int{V: 0}
	out, err := goTest(t, calcFiles(t, p))
	if err == nil {
		t.Fatal("go test passed with a wrong expectation")
	}
	for _, want := range []string{
		"add(a=1, b=2) = 3 [], canon says 4 []",
		fmt.Sprintf("add(a=9223372036854775807, b=1) = 0 [%s], canon says 0 []", eOverflow),
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

// goTest writes files into a module of the installed Go, then runs gofmt -s -l, go vet and go test -race.
func goTest(t *testing.T, files map[string][]byte) ([]byte, error) {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	dir := t.TempDir()
	files["go.mod"] = []byte("module " + calcModule + "\n\ngo " + goVersion(t) + "\n")
	for _, p := range sortedPaths(files) {
		path := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, files[p], 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out := run(t, dir, exec.Command("gofmt", "-s", "-l", ".")); len(out) > 0 {
		t.Errorf("gofmt -s would change:\n%s", out)
	}
	run(t, dir, exec.Command("go", "vet", "./..."))
	cmd := exec.Command("go", "test", "-race", "-count=1", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
	return cmd.CombinedOutput()
}
