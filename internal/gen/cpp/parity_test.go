package cppgen_test

import (
	"bytes"
	"go/version"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
)

const parityModule = "example.com/parity"

// parityPackage is the constructs fixture with a Go data emit beside its C++ one: one schema,
// two loaders. Package fns have no key in these files and are left out.
func parityPackage() (*ir.Package, *ir.Emit) {
	p := constructs()
	p.Fns = nil
	e := &ir.Emit{Target: ir.TargetGo, Out: "out/go/", Dir: "demo/shop/out/go", GoImport: parityModule + "/shop", Mode: ir.ModeData, GoPackage: "shop"}
	p.Emits = append(p.Emits, e)
	return p, e
}

// log-2026-09-24 "Loader parity": the Go and C++ loaders of one schema turn the same files,
// canon-written or edited by hand, into byte-identical lines, and those are the vocabulary's.
func TestLoaderParity(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated Go and C++")
	}
	p, goEmit := parityPackage()
	dir := t.TempDir()
	cases := strictCases()
	args, wants := make([]string, len(cases)), make([]string, len(cases))
	for i, c := range cases {
		args[i], wants[i] = c.write(t, dir)
	}
	check := func(target, out string) {
		got := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if len(got) != len(cases) {
			t.Fatalf("%s: %d lines for %d cases:\n%s", target, len(got), len(cases), out)
		}
		for i, c := range cases {
			if got[i] != wants[i] {
				t.Errorf("%s: %s (%s):\n got %s\nwant %s", target, c.name, c.rule, got[i], wants[i])
			}
		}
	}
	check("go", runGo(t, p, goEmit, args))
	writeTree(t, dir, generate(t, p), "strict_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"shop.gen.cpp", "main.cpp"}, args...) {
		check("c++", out)
	}
}

// runGo builds the Go loaders of p and testdata/main/strict_main.go into one module and runs
// the driver with args.
func runGo(t *testing.T, p *ir.Package, e *ir.Emit, args []string) string {
	t.Helper()
	files, err := gogen.Generate(p, e)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(path string, content []byte) {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", []byte("module "+parityModule+"\n\ngo "+strings.TrimPrefix(version.Lang(runtime.Version()), "go")+"\n"))
	for _, f := range files {
		write(e.GoPackage+"/"+f.Path, f.Content)
	}
	copyFile(t, filepath.Join("testdata", "main", "strict_main.go"), filepath.Join(dir, "main.go"), same)
	bin := filepath.Join(dir, "driver")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	var stdout, stderr bytes.Buffer
	run := exec.Command(bin, args...)
	run.Stdout, run.Stderr = &stdout, &stderr
	if err := run.Run(); err != nil {
		t.Fatalf("go driver: %v\n%s%s", err, stdout.String(), stderr.String())
	}
	return stdout.String()
}
