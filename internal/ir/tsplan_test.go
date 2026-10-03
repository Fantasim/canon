package ir_test

import (
	"regexp"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"

	tsgen "github.com/fantasim/canonlang/internal/gen/ts"
	"github.com/fantasim/canonlang/internal/ir"
)

var importLine = regexp.MustCompile(`(?m)^import (?:type )?\{ ([^}]*) \} from `)

// TestTSPlanReservesImports is CODEGEN.md §2.8, §3.5 and DECISIONS 278: every name gen/ts imports into a module is in the module scope of the TS name plan, so a declaration taking it is E8005 from check (testdata/findings/E8005_45.txtar), never a crash of the build.
func TestTSPlanReservesImports(t *testing.T) {
	ar, err := txtar.ParseFile("testdata/tsplan/imports.txtar")
	if err != nil {
		t.Fatal(err)
	}
	w := newWorld(t)
	for _, f := range ar.Files {
		w.add(t, f.Name, f.Data)
	}
	w.calls = w.fixtureCalls
	imported := 0
	for _, p := range w.build(t) {
		for _, e := range p.Emits {
			if e.Target == ir.TargetTS {
				imported += checkTSImports(t, w, p, e)
			}
		}
	}
	if want := 20; imported < want {
		t.Errorf("%d imported names, want at least %d: the case no longer exercises every import", imported, want)
	}
}

// checkTSImports generates one ts emit and checks that its plan reserves every name it imports; it returns how many it imports.
func checkTSImports(t *testing.T, w *world, p *ir.Package, e *ir.Emit) int {
	t.Helper()
	fakeVectors(p)
	files, err := tsgen.Generate(p, e)
	if err != nil {
		t.Fatalf("%s: %v\n%s", p.Name, err, w.findings(t))
	}
	module := ir.TSModuleNames(p, e)
	var names []string
	for _, m := range importLine.FindAllStringSubmatch(string(files[0].Content), -1) {
		names = append(names, strings.Split(m[1], ", ")...)
	}
	for _, name := range names {
		if !module[name] {
			t.Errorf("%s imports %s, which its TS name plan does not reserve", p.Name, name)
		}
	}
	return len(names)
}
