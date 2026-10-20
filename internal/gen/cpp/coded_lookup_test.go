package cppgen_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// codedLookupPair is demo.codes, a baked package holding a @codes enum whose codes are not in declaration order, and demo.uses, a baked package whose constexpr lookup (an Int) and stored lookup (a list) take that enum (CODEGEN.md §2.8, §5.2, §5.10; DECISIONS 383).
func codedLookupPair() (codes, uses *ir.Package) {
	element := &ir.Enum{Pkg: "demo.codes", Name: "Element", Codes: &tU8, Members: members([]string{"FIRE", "FIRE", "WATER", "WATER", "EARTH", "EARTH"}, []int64{3, 1, 200})}
	codesEmit := &ir.Emit{Target: ir.TargetCpp, Dir: "demo/codes/out", Mode: ir.ModeBaked, Namespace: "demo::codes"}
	codes = &ir.Package{Name: "demo.codes", Dir: "demo/codes", Types: []ir.Type{element}, Emits: []*ir.Emit{codesEmit}}
	byElement := []*ir.Param{{Name: "e", Type: ir.TypeRef{Kind: types.Enum, Named: element}}}
	usesEmit := &ir.Emit{Target: ir.TargetCpp, Dir: "demo/uses/out", Mode: ir.ModeBaked, Namespace: "demo::uses"}
	uses = &ir.Package{
		Name: "demo.uses", Dir: "demo/uses",
		Fns: []*ir.ExportFn{
			lookup("weightOf", "", tInt, byElement, num(4), num(1), num(9)),
			lookup("labelsOf", "", listOf(tString), byElement, list(str("fire")), list(str("water"), str("wet")), list()),
		},
		Imports: []*ir.PackageRef{{Name: codes.Name, Dir: codes.Dir, Emits: codes.Emits}},
		Emits:   []*ir.Emit{usesEmit},
	}
	return codes, uses
}

// codedLookupWant is what coded_lookup_main.cpp prints: each member's cells, read through detail::ElementOrdinal of the importing package's enum.
const codedLookupWant = "FIRE 4 [fire]\nWATER 1 [water wet]\nEARTH 9 []\n"

// DECISIONS 383, CODEGEN.md §5.2, §5.10: lookups over another package's @codes enum call its detail::<E>Ordinal, constexpr and stored, with every compiler and mode.
func TestImportedCodedLookupsCompileAndRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	codes, uses := codedLookupPair()
	for _, p := range []*ir.Package{codes, uses} {
		out := filepath.Join(dir, filepath.FromSlash(cppEmit(p).Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, out, generate(t, p))
	}
	copyFile(t, filepath.Join("testdata", "main", "coded_lookup_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	sources := []string{"demo/codes/out/codes.gen.cpp", "demo/uses/out/uses.gen.cpp", "main.cpp"}
	for _, got := range buildAndRun(t, dir, sources) {
		if got != codedLookupWant {
			t.Errorf("got:\n%s\nwant:\n%s", got, codedLookupWant)
		}
	}
}
