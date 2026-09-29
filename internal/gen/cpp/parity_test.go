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

// nestParityPackage is nestPackage with a Go data emit beside its C++ one (log-2026-09-24 A1: duplicate ids of every keyable() type, TYPES.md §9.1).
func nestParityPackage() (*ir.Package, *ir.Emit) {
	p := nestPackage()
	e := &ir.Emit{Target: ir.TargetGo, Out: "out/go/", Dir: "demo/nest/out/go", GoImport: parityModule + "/nest", Mode: ir.ModeData, GoPackage: "nest"}
	p.Emits = append(p.Emits, e)
	return p, e
}

// parityUnit is one package's C++ sources and Go build input for TestLoaderParity.
type parityUnit struct {
	pkg     *ir.Package
	goEmit  *ir.Emit
	cppFile string
}

// log-2026-09-24 "Loader parity", A1 and A1 C++ Float32: the Go and C++ loaders of demo.shop and demo.nest turn the same files, canon-written or edited by hand (some loaded by the C++ driver under a ',' locale), into byte-identical lines, and those are the vocabulary's.
func TestLoaderParity(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated Go and C++")
	}
	shopP, shopGo := parityPackage()
	nestP, nestGo := nestParityPackage()
	units := []parityUnit{{shopP, shopGo, "shop.gen.cpp"}, {nestP, nestGo, "nest.gen.cpp"}}
	dir := t.TempDir()
	cases := append(append(append([]strictCase{}, strictCases()...), nestCases()...), float32Cases()...)
	if loc, err := buildCommaLocale(t); err != nil {
		t.Logf("no ',' locale can be built here, its cases are left out: %v", err)
		cases = withoutMode(cases, commaLocale)
	} else {
		t.Setenv("LOCPATH", loc)
	}
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
	check("go", runGo(t, units, "strict_main.go", args))
	sources := []string{"main.cpp"}
	for _, u := range units {
		writeFiles(t, dir, generate(t, u.pkg))
		sources = append(sources, u.cppFile)
	}
	copyFile(t, filepath.Join("testdata", "main", "strict_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	for _, out := range buildAndRun(t, dir, sources, args...) {
		check("c++", out)
	}
}

// runGo builds every unit's Go loaders and the driver testdata/main/<driver> into one module and
// runs it with args.
func runGo(t *testing.T, units []parityUnit, driver string, args []string) string {
	t.Helper()
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
	for _, u := range units {
		files, err := gogen.Generate(u.pkg, u.goEmit)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			write(u.goEmit.GoPackage+"/"+f.Path, f.Content)
		}
	}
	copyFile(t, filepath.Join("testdata", "main", driver), filepath.Join(dir, "main.go"), same)
	bin := filepath.Join(dir, exe("driver"))
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
