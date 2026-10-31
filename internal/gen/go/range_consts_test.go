package gogen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// boundBlock is the start of Probe's const block with its T19 comments (CODEGEN.md §2.5, §5.15), in field order, then level order, Min first.
const boundBlock = `const (
	// Probe_NMin is the lower bound of n, from its refinement 1..=120.
	Probe_NMin = 1
	// Probe_NMax is the upper bound of n, from its refinement 1..=120.
	Probe_NMax = 120
	// Probe_U8Min is the lower bound of u8, from its refinement 0..=255.
	Probe_U8Min = 0
	// Probe_U8Max is the upper bound of u8, from its refinement 0..=255.
	Probe_U8Max = 255
	// Probe_I8Min is the lower bound of i8, from its refinement 0..=300.
	Probe_I8Min = 0
	// Probe_I8Max is the upper bound of i8, from its refinement 0..=300.
	Probe_I8Max = 300
	// Probe_ExEnd is the exclusive upper bound of ex, from its refinement ..5.
	Probe_ExEnd = 5
	// Probe_FMin is the lower bound of f, from its refinement 0..1.
	Probe_FMin float64 = 0
	// Probe_FEnd is the exclusive upper bound of f, from its refinement 0..1.
	Probe_FEnd float64 = 1
	// Probe_F32Max is the upper bound of f32, from its refinement ..=0.1.
	Probe_F32Max float64 = 0.1
	// Probe_DMin is the lower bound of d, from its refinement 1s..=1h.
	Probe_DMin = 1000 * time.Millisecond
	// Probe_DMax is the upper bound of d, from its refinement 1s..=1h.
	Probe_DMax = 3600000 * time.Millisecond
	// Probe_SLenMin is the lower bound of the byte length of s, from its refinement 1..=4.
	Probe_SLenMin = 1
	// Probe_SLenMax is the upper bound of the byte length of s, from its refinement 1..=4.
	Probe_SLenMax = 4
	// Probe_TagsLenMax is the upper bound of the element count of tags, from its refinement ..=3.
	Probe_TagsLenMax = 3
	// Probe_TagsElemMin is the lower bound of each element of tags, from its refinement 0..=5.
	Probe_TagsElemMin = 0
`

// boundLines are bound constants further down Probe's block: a map's key and value, an optional element, a union's base, pairs, bits and a keyed list.
var boundLines = []string{
	"\t// Probe_MKeyLenMin is the lower bound of the byte length of each key of m, from its refinement 1..=3.\n\tProbe_MKeyLenMin = 1\n",
	"\t// Probe_MValueMax is the upper bound of each value of m, from its refinement 0..=9.\n\tProbe_MValueMax = 9\n",
	"\t// Probe_MiKeyMax is the upper bound of each key of mi, from its refinement ..=100.\n\tProbe_MiKeyMax = 100\n",
	"\t// Probe_OptsElemMax is the upper bound of each element of opts, from its refinement 0..=5.\n\tProbe_OptsElemMax = 5\n",
	"\t// Probe_LitLenMin is the lower bound of the byte length of lit, from its refinement 6..=8.\n\tProbe_LitLenMin = 6\n",
	"\t// Probe_RowsLenMax is the upper bound of the element count of rows, from its refinement ..=2.\n\tProbe_RowsLenMax = 2\n",
	"\t// Probe_FlagsLenMax is the upper bound of the element count of flags, from its refinement 1..=2.\n\tProbe_FlagsLenMax = 2\n",
	"\t// Probe_SlotsLenMin is the lower bound of the element count of slots, from its refinement 1..=2.\n\tProbe_SlotsLenMin = 1\n",
}

// runSmokes vets and tests a module of the files, the smoke tests (each copied to a file of the same name) and the data files.
func runSmokes(t *testing.T, files map[string][]byte, pkgDir string, smokes []string, data map[string][]byte) {
	t.Helper()
	dir := writeModule(t, files, pkgDir, "", data)
	for _, s := range smokes {
		src, err := os.ReadFile(s)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(pkgDir), filepath.Base(s)), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out := run(t, dir, exec.Command("gofmt", "-s", "-l", ".")); len(out) > 0 {
		t.Errorf("gofmt -s would change:\n%s", out)
	}
	run(t, dir, exec.Command("go", "vet", "./..."))
	run(t, dir, exec.Command("go", "test", "-count=1", "./..."))
}

// CODEGEN.md §5.15, §3.3, T19, DECISIONS 400: each finite side of a field's refinement is a constant of its type's name, in one const block after the struct, before its getters; an array length, a bound of len, a float64 bound and a time.Duration bound compile.
func TestGoBoundConstants(t *testing.T) {
	files := generateData(t, rangePackage())
	src := generatedSource(files)
	if !strings.Contains(src, boundBlock) {
		t.Errorf("no Probe const block:\n%s", src)
	}
	for _, want := range boundLines {
		if !strings.Contains(src, want) {
			t.Errorf("no %q", want)
		}
	}
	if st, first, block := strings.Index(src, "type Probe struct"), strings.Index(src, "func (self *Probe)"), strings.Index(src, boundBlock); !(st < block && block < first) {
		t.Errorf("the const block is not between the struct (%d) and its getters (%d): %d", st, first, block)
	}
	runSmokes(t, files, rangeDir, []string{"testdata/smoke/range_consts_test.go"}, nil)
}
