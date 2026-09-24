package gogen_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	constructsIR     = "testdata/ir/constructs.json"
	constructsGolden = "testdata/constructs"
)

// listedCells is for fixtures whose lookup tables are all written in the JSON.
func listedCells(w *world, fn *ir.ExportFn) []value.Value {
	w.t.Fatalf("fixture: %s has no cells", fn.Name)
	return nil
}

// CODEGEN.md §5.1–§5.10: every other construct of baked Go equals its golden.
func TestConstructsGolden(t *testing.T) {
	w := loadWorld(t, constructsIR, listedCells)
	files := generate(t, w)
	checkGoldens(t, files, sortedPaths(files), constructsGolden)
}

// CODEGEN.md §9, decision 205: the constructs compile with the installed Go and read back their values.
func TestConstructsCompile(t *testing.T) {
	w := loadWorld(t, constructsIR, listedCells)
	files := generate(t, w)
	compile(t, w, files, sortedPaths(files), [][2]string{{"constructs/out/go/smoke_test.go", "testdata/smoke/constructs_test.go"}})
}

const (
	keysIR     = "testdata/ir/keys.json"
	keysGolden = "testdata/keys"
)

// CODEGEN.md §5.8: a ref into a value the emit leaves out has a key getter only.
func TestKeysGolden(t *testing.T) {
	w := loadWorld(t, keysIR, listedCells)
	files := generate(t, w)
	checkGoldens(t, files, sortedPaths(files), keysGolden)
}

// CODEGEN.md §9: the key-only getters compile and read back their keys.
func TestKeysCompile(t *testing.T) {
	w := loadWorld(t, keysIR, listedCells)
	files := generate(t, w)
	compile(t, w, files, sortedPaths(files), [][2]string{{"keys/out/go/smoke_test.go", "testdata/smoke/keys_test.go"}})
}

// CODEGEN.md §4.4: a Never? field has no storage and no getter.
func TestNeverFieldOmitted(t *testing.T) {
	w := loadWorld(t, constructsIR, listedCells)
	files := generate(t, w)
	src := string(files["constructs/out/go/constructs.gen.go"])
	if src == "" || strings.Contains(src, "unused") || strings.Contains(src, "Unused") {
		t.Errorf("the Never? field Potion.unused is emitted, or the file is missing")
	}
}

const (
	escapesIR     = "testdata/ir/escapes.json"
	escapesGolden = "testdata/escapes"
)

// CODEGEN.md §3.4, decision 182: parameters and locals named like an import are escaped.
func TestEscapesGolden(t *testing.T) {
	w := loadWorld(t, escapesIR, listedCells)
	files := generate(t, w)
	checkGoldens(t, files, sortedPaths(files), escapesGolden)
	src := string(files["escapes/out/go/escapes.gen.go"])
	for _, want := range []string{"func F(q_ q.Q) bool", "func GateOf(d_ d.D, q_ q.Q) *Gate", "d_ := &escapesData{}"} {
		if !strings.Contains(src, want) {
			t.Errorf("escapes.gen.go has no %q", want)
		}
	}
}

// CODEGEN.md §9: the escaped names compile and read back their values.
func TestEscapesCompile(t *testing.T) {
	w := loadWorld(t, escapesIR, listedCells)
	files := generate(t, w)
	compile(t, w, files, sortedPaths(files), [][2]string{{"escapes/out/go/smoke_test.go", "testdata/smoke/escapes_test.go"}})
}
