package gogen_test

import (
	"bytes"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/fnsize"
)

const (
	partsGoldens   = "testdata"
	buildPartsDir  = "build-parts"
	decodePartsDir = "decode-parts"
	rowsFile       = "rows/rows.gen.go"
	wideFile       = "wide/wide.gen.go"
	buildFn        = "buildRows"
	decodeFn       = "decodeWide"
	cutLimit       = 1 // one unit to a part: the smallest budget there is
	smallBuild     = 24
	smallDecode    = 40
	sweepRows      = 40
	sweepFields    = 40
	buildLeast     = 10 // a row's statement is at most 7 lines
	decodeLeast    = 25 // a field's unit is at most 17 lines, and a part adds its return
	hugeRows       = 10000
	hugeFields     = 300
	buildPartsMin  = 100
	decodePartsMin = 2
	// the parameters of a part holding an inline variant's field
	inlinePartParams = "path string, raw json.RawMessage, obj map[string]json.RawMessage, out *"
)

// cutAt is the generator with a budget of limit lines for a function's body (CODEGEN.md §2.11).
func cutAt(limit int) generator {
	return func(p *ir.Package, e *ir.Emit) ([]ir.File, error) { return gogen.GenerateWithPartLines(p, e, limit) }
}

// partFuncs measures the functions of a generated file by name.
func partFuncs(t *testing.T, src []byte) map[string]fnsize.Func {
	t.Helper()
	funcs, err := fnsize.Go(string(src))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]fnsize.Func{}
	for _, f := range funcs {
		out[f.Name] = f
	}
	return out
}

// partsOf are fn's parts, fn_0, fn_1, … in order, until one is missing.
func partsOf(funcs map[string]fnsize.Func, fn string) []fnsize.Func {
	var parts []fnsize.Func
	for n := 0; ; n++ {
		f, ok := funcs[ir.GoPart(fn, n)]
		if !ok {
			return parts
		}
		parts = append(parts, f)
	}
}

// fits reports a function within the budget: its body is at most limit lines, or it is one statement, which is never cut (CODEGEN.md §2.11).
func fits(f fnsize.Func, limit int) bool { return f.Body <= limit || f.Statements == 1 }

// CODEGEN.md §2.11, §5.9, DECISIONS 435: a baked package's build is cut into build<Last>_<N> parts the build calls in order, each holding whole statements; equals its golden.
func TestBuildIsCut(t *testing.T) {
	files := generateWith(t, cutAt(smallBuild), loadFixture(t, partsRowsFixture(3)))
	checkGoldens(t, files, sortedPaths(files), partsGoldens+"/"+buildPartsDir)
	funcs := partFuncs(t, files[rowsFile])
	parts := partsOf(funcs, buildFn)
	if len(parts) < 2 {
		t.Fatalf("%s has %d parts, want at least 2", buildFn, len(parts))
	}
	for _, part := range parts {
		if !fits(part, smallBuild) {
			t.Errorf("%s has %d lines, over the budget of %d", part.Name, part.Body, smallBuild)
		}
	}
}

// CODEGEN.md §2.11, §5.13, DECISIONS 435: a class's decoder is cut into decode<T>_<N> parts that take the object and the value, each holding whole fields; equals its golden.
func TestDecodeIsCut(t *testing.T) {
	files := generateWith(t, cutAt(smallDecode), loadFixture(t, partsWideFixture(5)))
	checkGoldens(t, files, sortedPaths(files), partsGoldens+"/"+decodePartsDir)
	parts := partsOf(partFuncs(t, files[wideFile]), decodeFn)
	if len(parts) < 2 {
		t.Fatalf("%s has %d parts, want at least 2", decodeFn, len(parts))
	}
	for _, part := range parts {
		if !fits(part, smallDecode) {
			t.Errorf("%s has %d lines, over the budget of %d", part.Name, part.Body, smallDecode)
		}
	}
}

// partsSweep is one function the boundary test cuts at every budget.
type partsSweep struct {
	file, fn string
	least    int
	fixture  *fixture
}

// sweeps are the build of a table and the decoder of a record, each cut at every budget from its least.
var sweeps = []partsSweep{
	{rowsFile, buildFn, buildLeast, partsRowsFixture(sweepRows)},
	{wideFile, decodeFn, decodeLeast, partsWideFixture(sweepFields)},
}

// CODEGEN.md §2.11, DECISIONS 435: a body of exactly the budget is not cut, byte for byte what Generate writes; one line over is; no part holds more than the budget, and some part holds exactly it (overhead counted to the line, measured by fnsize).
func TestCutIsExactAtTheBoundary(t *testing.T) {
	for _, c := range sweeps {
		w := loadFixture(t, c.fixture)
		uncut := generateWith(t, gogen.Generate, w)
		whole := partFuncs(t, uncut[c.file])[c.fn].Body
		if whole <= c.least {
			t.Fatalf("%s has %d lines: too short to sweep", c.fn, whole)
		}
		if got := partsOf(partFuncs(t, uncut[c.file]), c.fn); len(got) > 0 {
			t.Errorf("%s of %d lines is cut at the default budget", c.fn, whole)
		}
		if at := generateWith(t, cutAt(whole), w); !maps.EqualFunc(at, uncut, bytes.Equal) {
			t.Errorf("%s: a budget of exactly its %d lines cuts it", c.fn, whole)
		}
		if !c.sweepBudgets(t, w, whole) {
			t.Errorf("%s: no part fills its budget: the overhead is counted too high", c.fn)
		}
	}
}

// sweepBudgets cuts the function at every budget below its whole size, checks every part against the budget, and reports whether some part filled it exactly.
func (c partsSweep) sweepBudgets(t *testing.T, w *world, whole int) bool {
	t.Helper()
	tight := false
	for limit := c.least; limit < whole; limit++ {
		parts := partsOf(partFuncs(t, generateWith(t, cutAt(limit), w)[c.file]), c.fn)
		if len(parts) == 0 {
			t.Fatalf("%s of %d lines is not cut at a budget of %d", c.fn, whole, limit)
		}
		for _, part := range parts {
			if !fits(part, limit) {
				t.Errorf("%s has %d lines at a budget of %d", part.Name, part.Body, limit)
			}
			tight = tight || part.Body == limit
		}
	}
	return tight
}

// CODEGEN.md §2.11, DECISIONS 435: at the real budget, a record of 300 members and a table of 10,000 rows have no function over 1,000 lines but a one-statement one (measured by fnsize); the build and the decoder are cut into parts.
func TestHugeFixtureStaysInTheBudget(t *testing.T) {
	w := loadFixture(t, partsFixture(hugeRows, hugeFields))
	for _, p := range w.pkgs {
		files, err := gogen.Generate(p, p.Emits[0])
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f.Path, genSuffix) {
				checkHugeFile(t, f)
			}
		}
	}
}

// checkHugeFile measures every function of a generated file against the budget and counts the parts of the two big ones.
func checkHugeFile(t *testing.T, f ir.File) {
	t.Helper()
	funcs := partFuncs(t, f.Content)
	for _, name := range slices.Sorted(maps.Keys(funcs)) {
		if fn := funcs[name]; !fits(fn, ir.PartLines) {
			t.Errorf("%s: %s has %d lines", f.Path, name, fn.Body)
		}
	}
	checkPartCount(t, f.Path, funcs)
}

// checkPartCount asserts the huge fixture's two big functions are cut into enough parts.
func checkPartCount(t *testing.T, path string, funcs map[string]fnsize.Func) {
	t.Helper()
	for _, c := range []struct {
		file, fn string
		least    int
	}{{rowsFile, buildFn, buildPartsMin}, {wideFile, decodeFn, decodePartsMin}} {
		if strings.HasSuffix(path, c.file) {
			if n := len(partsOf(funcs, c.fn)); n < c.least {
				t.Errorf("%s has %d parts, want at least %d", c.fn, n, c.least)
			}
		}
	}
}

// CODEGEN.md §2.11, DECISIONS 435: with one unit to a part, every package of the golden suites still generates, formats and answers as it did: the parts of shop's data mode (every field kind, an inline variant that reads the whole object) and of the baked constructs, teamboard, escapes and emberfall packages vet and run their smoke tests.
func TestCutPackagesRun(t *testing.T) {
	cut := cutAt(cutLimit)
	t.Run("shop", func(t *testing.T) {
		files := generateDataWith(t, cut, base(), newShop())
		requireAnyPart(t, files)
		if !strings.Contains(string(files["demo/shop/out/go/shop.gen.go"]), inlinePartParams) {
			t.Errorf("no part of shop's decoders takes the whole document for the inline variant")
		}
		runData(t, files, "demo/shop/out/go", "testdata/smoke/data_shop_test.go", readData(t, "testdata/datafiles/shop"))
	})
	for name, mode := range map[string]ir.Mode{"ember-baked": ir.ModeBaked, "ember-data": ir.ModeData} { //canon:unordered each subtest alone
		t.Run(name, func(t *testing.T) {
			c, w := emberfall(mode)
			files := generateDataWith(t, cut, c.p, w)
			requireAnyPart(t, files)
			smoke, data := "testdata/smoke/emberfall_baked_test.go", map[string][]byte(nil)
			if mode == ir.ModeData {
				smoke, data = "testdata/smoke/emberfall_data_test.go", readData(t, "testdata/datafiles/emberfall")
			}
			runData(t, files, emberWorld, smoke, data)
		})
	}
	for _, c := range []struct{ name, ir, smoke, dest string }{
		{"constructs", constructsIR, "testdata/smoke/constructs_test.go", "constructs/out/go/smoke_test.go"},
		{"keys", keysIR, "testdata/smoke/keys_test.go", "keys/out/go/smoke_test.go"},
		{"escapes", escapesIR, "testdata/smoke/escapes_test.go", "escapes/out/go/smoke_test.go"},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := loadWorld(t, c.ir, listedCells)
			files := generateWith(t, cut, w)
			requireAnyPart(t, files)
			compile(t, w, files, sortedPaths(files), [][2]string{{c.dest, c.smoke}})
		})
	}
}

// partName matches the name of a part: build<Last>_<N> or decode<T>_<N>.
var partName = regexp.MustCompile(`^(build|decode)\w*_\d+$`)

// requireAnyPart fails when no generated file holds a function named like a part.
func requireAnyPart(t *testing.T, files map[string][]byte) {
	t.Helper()
	for _, path := range sortedPaths(files) {
		if !strings.HasSuffix(path, genSuffix) {
			continue
		}
		for name := range partFuncs(t, files[path]) { //canon:unordered one found is enough
			if partName.MatchString(name) {
				return
			}
		}
	}
	t.Error("no generated file has a part")
}

// requireParts fails when the generated file at path holds no part of fn.
func requireParts(t *testing.T, files map[string][]byte, path, fn string) {
	t.Helper()
	if len(partsOf(partFuncs(t, files[path]), fn)) == 0 {
		t.Errorf("%s has no part of %s", path, fn)
	}
}
