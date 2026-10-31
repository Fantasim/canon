package tsgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/tsc"
)

const (
	boundsGood    = "use_good.ts"
	boundsBad     = "use_bad.ts"
	boundsMissing = "use_missing.ts"
	notAssignable = "TS2322"
	notExported   = "has no exported member"
)

// boundLines are Bounds' constants (testdata/ts/range_bounds.txtar) with their T19 comments (CODEGEN.md §2.5, §5.15): a bigint at a @ts(bigint) position, a number elsewhere, a Duration in milliseconds, a length, a level below the field, a case's field.
var boundLines = []string{
	"/** The lower bound of big, from its refinement 1..=10000000000000000. */\nexport const Bounds_BigMin: bigint = 1n;\n",
	"/** The upper bound of big, from its refinement 1..=10000000000000000. */\nexport const Bounds_BigMax: bigint = 10000000000000000n;\n",
	"/** The lower bound of huge, from its refinement 0..=10000000000000000. */\nexport const Bounds_HugeMin: number = 0;\n",
	"/** The exclusive upper bound of each element of bigs, from its refinement ..5. */\nexport const Bounds_BigsElemEnd: bigint = 5n;\n",
	"/** The upper bound of each key of bigk, from its refinement 0..=5. */\nexport const Bounds_BigkKeyMax: bigint = 5n;\n",
	"/** The lower bound of f, from its refinement -1.5..=1.5. */\nexport const Bounds_FMin: number = -1.5;\n",
	"/** The upper bound of f32, from its refinement ..=0.1. */\nexport const Bounds_F32Max: number = 0.1;\n",
	"/** The upper bound of d, from its refinement 1s..=1h. */\nexport const Bounds_DMax: number = 3600000;\n",
	"/** The lower bound of the byte length of s, from its refinement 1..=4. */\nexport const Bounds_SLenMin: number = 1;\n",
	"/** The upper bound of the element count of tags, from its refinement ..=3. */\nexport const Bounds_TagsLenMax: number = 3;\n",
	"/** The upper bound of each element of tags, from its refinement 0..=5. */\nexport const Bounds_TagsElemMax: number = 5;\n",
	"/** The lower bound of the byte length of each key of m, from its refinement 1..=3. */\nexport const Bounds_MKeyLenMin: number = 1;\n",
	"/** The upper bound of each value of m, from its refinement 0..=9. */\nexport const Bounds_MValueMax: number = 9;\n",
	"/** The upper bound of each element of opts, from its refinement 0..=5. */\nexport const Bounds_OptsElemMax: number = 5;\n",
	"/** The exclusive upper bound of ex, from its refinement ..5. */\nexport const Bounds_ExEnd: number = 5;\n",
	"/** The upper bound of r, from its refinement 0..=2. */\nexport const ShapeCircle_RMax: number = 2;\n",
}

// boundUse imports constants of the generated module and gives them the type the spec says: a bigint, a number.
const boundUse = `import { Bounds_BigMax, Bounds_TagsLenMax, Bounds_FMin, Bounds_DMax, ShapeCircle_RMax } from "./a.js";
export const big: bigint = Bounds_BigMax;
export const count: number = Bounds_TagsLenMax + Bounds_DMax + ShapeCircle_RMax + Bounds_FMin;
`

// readFile is the text of the file at path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// CODEGEN.md §5.15, §3.3, T19, DECISIONS 400: each finite side of a field's refinement is an exported constant of its type's name after its interface; a bigint at a @ts(bigint) position, a number elsewhere, and none for a number bound past 2^53, whose name stays reserved.
func TestTSBoundConstants(t *testing.T) {
	tc := tsc.Find(t)
	dir := t.TempDir()
	paths := writeOutputs(t, dir, rangeWorld(t, "range_bounds").generate(t))
	body := readFile(t, paths[0])
	for _, want := range boundLines {
		if !strings.Contains(body, want) {
			t.Errorf("no %q", want)
		}
	}
	if strings.Contains(body, "Bounds_HugeMax") {
		t.Error("Bounds_HugeMax is written, its bound is past 2^53")
	}
	if i, j := strings.Index(body, "export interface Bounds {"), strings.Index(body, "export const Bounds_BigMin"); i < 0 || j < i {
		t.Errorf("the constants are not after the interface: %d, %d", i, j)
	}
	module := filepath.Dir(paths[0])
	writeFile(t, filepath.Join(module, boundsGood), []byte(boundUse))
	writeFile(t, filepath.Join(module, boundsBad), []byte("import { Bounds_BigMax } from \"./a.js\";\nexport const bad: number = Bounds_BigMax;\n"))
	writeFile(t, filepath.Join(module, boundsMissing), []byte("import { Bounds_HugeMax } from \"./a.js\";\nexport const missing = Bounds_HugeMax;\n"))
	for _, c := range tc.Compilers {
		args := func(f string) []string {
			return append([]string{c.Script, "--noEmit"}, tsc.Args(tc.Typings, paths[0], filepath.Join(module, f))...)
		}
		if out, err := run(tc, args(boundsGood), ""); err != nil {
			t.Errorf("%s: %v\n%s", c.Name, err, out)
		}
		if out, err := run(tc, args(boundsBad), ""); err == nil || !strings.Contains(out, notAssignable) {
			t.Errorf("%s: a bigint bound as a number: err %v, want %s\n%s", c.Name, err, notAssignable, out)
		}
		if out, err := run(tc, args(boundsMissing), ""); err == nil || !strings.Contains(out, notExported) {
			t.Errorf("%s: an unsafe bound: err %v, want %s\n%s", c.Name, err, notExported, out)
		}
	}
}
