package gogen_test

import (
	"bytes"
	"flag"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
)

var update = flag.Bool("update", false, "rewrite the goldens under testdata/ instead of comparing")

// generate runs the generator twice on every package of w: equal outputs (CODEGEN.md §2.7).
func generate(t *testing.T, w *world) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, p := range w.pkgs {
		e := p.Emits[0]
		first, err := gogen.Generate(p, e)
		if err != nil {
			t.Fatalf("%s: %v", p.Name, err)
		}
		again, err := gogen.Generate(p, e)
		if err != nil {
			t.Fatalf("%s: %v", p.Name, err)
		}
		for i, f := range first {
			if !bytes.Equal(f.Content, again[i].Content) {
				t.Errorf("%s: %s differs between two runs", p.Name, f.Path)
			}
			out[modulePath(t, w, e)+"/"+f.Path] = f.Content
		}
	}
	return out
}

// modulePath is an emit's directory inside the fixture's module.
func modulePath(t *testing.T, w *world, e *ir.Emit) string {
	t.Helper()
	rel, ok := strings.CutPrefix(e.GoImport, w.fx.Module+"/")
	if !ok {
		t.Fatalf("%s is not in module %s", e.GoImport, w.fx.Module)
	}
	return rel
}

// checkGoldens compares the generated files with root/<path>, or rewrites them with -update.
func checkGoldens(t *testing.T, files map[string][]byte, paths []string, root string) {
	t.Helper()
	for _, p := range paths {
		path := filepath.Join(root, filepath.FromSlash(p))
		if *update {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, files[p], 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%v (run go test -update)", err)
		}
		if !bytes.Equal(files[p], want) {
			t.Errorf("%s differs from its golden (run go test -update and review the diff)", path)
		}
	}
}

// sortedPaths is the generated paths in byte order.
func sortedPaths(files map[string][]byte) []string {
	return slices.Sorted(maps.Keys(files))
}

// compile runs gofmt -s -l, go vet and go test -bench in a go 1.23 module (CODEGEN.md §9).
func compile(t *testing.T, w *world, files map[string][]byte, paths []string, smoke [][2]string) {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	dir := t.TempDir()
	write := func(p string, content []byte) {
		path := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", []byte("module "+w.fx.Module+"\n\ngo 1.23\n"))
	for _, p := range paths {
		write(p, files[p])
	}
	for _, s := range smoke {
		src, err := os.ReadFile(s[1])
		if err != nil {
			t.Fatal(err)
		}
		write(s[0], src)
	}
	if out := run(t, dir, exec.Command("gofmt", "-s", "-l", ".")); len(out) > 0 {
		t.Errorf("gofmt -s would change:\n%s", out)
	}
	run(t, dir, exec.Command("go", "vet", "./..."))
	run(t, dir, exec.Command("go", "test", "-count=1", "-bench=.", "-benchtime=1x", "./..."))
}

func run(t *testing.T, dir string, cmd *exec.Cmd) []byte {
	t.Helper()
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", strings.Join(cmd.Args, " "), err, out)
	}
	return out
}
