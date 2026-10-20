package ir_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	goRunOptional = "testdata/e2e/gorun/optional_elems.txtar"
	goRunUnions   = "testdata/e2e/gorun/ref_unions.txtar"
	goRunForeign  = "testdata/e2e/gorun/foreign_ref_union.txtar"
	goRunKeys     = "testdata/e2e/gorun/foreign_keys.txtar"
	goRunPairs    = "testdata/e2e/gorun/pairs_unions.txtar"
	goRunLeaves   = "testdata/e2e/gorun/unchecked_keys.txtar"
	goRunVet      = "vet"
	goRunTest     = "test"
	goRunAll      = "./..."
	goRunNoCache  = "-count=1"
	goRunBinary   = "go"
)

// goRunCheck builds the project of the archive at path with no finding (cppRunBuild: the project, else e2eFeatures, built in a temp dir), then vets and tests the Go module its gen directory holds, whose probe_test.go reads the generated code back.
func goRunCheck(t *testing.T, path string) {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := cppRunBuild(t, path)
	gen := filepath.Join(dir, e2eGenRoot)
	if _, err := os.Stat(filepath.Join(gen, e2eGoModFile)); err != nil {
		writeGoMod(t, gen, e2eGoModule)
	}
	runIn(t, gen, exec.Command(goRunBinary, goRunVet, goRunAll))
	runIn(t, gen, exec.Command(goRunBinary, goRunTest, goRunNoCache, goRunAll))
}

// TestGoUncheckedKeysRun is CODEGEN.md §5.8 and DECISIONS 312, 385, 387 built and run: in an own class a go data loader checks a union or an optional ref where it checks a plain ref and reads one under a map value or at a second list level unchecked, as C++ does (TestCppUncheckedKeysRun).
func TestGoUncheckedKeysRun(t *testing.T) {
	t.Parallel()
	goRunCheck(t, goRunLeaves)
}

// TestGoPairsUnionsRun is DECISIONS 387 and CODEGEN.md §5.8, §5.14 built and run: a union over a ref in a pairs element record is checked by a go data loader like a plain ref there, at its slot key.
func TestGoPairsUnionsRun(t *testing.T) {
	t.Parallel()
	goRunCheck(t, goRunPairs)
}

// TestGoForeignKeysRun is CODEGEN.md §2.8 (Refs) and DECISIONS 385 and 387 built and run: a reader of another package's class refuses a ref naming no entry of the owner's keyed list in every position, its data reader a union's key too, with the loader text `no entry <key>`.
func TestGoForeignKeysRun(t *testing.T) {
	t.Parallel()
	goRunCheck(t, goRunKeys)
}

// TestGoForeignRefUnionRun is DECISIONS 387 and 323 (CODEGEN.md §2.8) built and run: another package's record holding a union over a ref into its baked table is read by a package's own reader, a data loader checking a key that is no literal against the owner's id enum, a types decoder reading it unchecked.
func TestGoForeignRefUnionRun(t *testing.T) {
	t.Parallel()
	goRunCheck(t, goRunForeign)
}

// TestGoRefUnionsRun is DECISIONS 387 (CODEGEN.md §4.1, §5.8, §5.13) built and run: a literal union over a ref is a Go string in baked, data and types mode; the data loader accepts a literal or checks the key as a plain ref, an enum key through its Parse, the types decoder reads it unchecked.
func TestGoRefUnionsRun(t *testing.T) {
	t.Parallel()
	goRunCheck(t, goRunUnions)
}

// TestGoOptionalElementsRun is DECISIONS 385 (CODEGEN.md §4.2, WIRE.md §5.4) built and run: a go emit holds optional list elements and map values as rt.List[*T] and rt.Map[K, *V] in baked, data and types mode alike, none written and read as nil, a present scalar written through rt.Ptr.
func TestGoOptionalElementsRun(t *testing.T) {
	t.Parallel()
	goRunCheck(t, goRunOptional)
}
