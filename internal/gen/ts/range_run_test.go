package tsgen_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/testkit/tsc"
)

const (
	rangeDriver   = "testdata/range/range.test.js"
	rangeRunsFile = "range-cases.json"
	rangeModule   = "features/a/a.js"
	rangeCaps     = "parseCapsFile"
)

// rangeRun is one line of range-cases.json: the driver calls fn of module on text and compares what it prints with want.
type rangeRun struct {
	Module string `json:"module"`
	Name   string `json:"name"`
	Fn     string `json:"fn"`
	Text   string `json:"text"`
	Want   string `json:"want"`
}

// within replaces from by to once in base; from empty keeps base.
func within(t *testing.T, base string, c rangeCase) string {
	t.Helper()
	if c.from == "" {
		return base
	}
	text := strings.Replace(base, c.from, c.to, 1)
	if text == base && c.from != c.to {
		t.Fatalf("%s: %q not in the base", c.name, c.from)
	}
	return text
}

// rangeWorld builds testdata/ts/<name>.txtar and returns its world.
func rangeWorld(t *testing.T, name string) *world {
	t.Helper()
	cases, err := golden.Load("testdata/ts/"+name+".txtar", golden.Expected(generatedTS))
	if err != nil {
		t.Fatal(err)
	}
	return buildWorld(t, cases[0].Archive.Files)
}

// runRange generates the cases' packages, type-checks them with every compiler, compiles them with the 5.0 floor and runs the driver on the runs, with node (CODEGEN.md §9).
func runRange(t *testing.T, names []string, runs []rangeRun) {
	t.Helper()
	tc := tsc.Find(t)
	src := t.TempDir()
	var paths []string
	for _, name := range names {
		paths = append(paths, writeOutputs(t, filepath.Join(src, name), rangeWorld(t, name).generate(t))...)
	}
	typecheck(t, tc, paths...)
	out := compile(t, tc, src, paths...)
	body, err := os.ReadFile(rangeDriver)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(out, filepath.Base(rangeDriver)), body)
	raw, err := json.Marshal(runs)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(out, rangeRunsFile), raw)
	runTests(t, tc, out)
}

// typesRuns are cases applied to base, parsed by fn of the module of the case named name.
func typesRuns(t *testing.T, name, base, fn string, cases []rangeCase) []rangeRun {
	t.Helper()
	var runs []rangeRun
	for _, c := range cases {
		runs = append(runs, rangeRun{name + "/" + rangeModule, c.name, fn, within(t, base, c), c.want})
	}
	return runs
}

// CODEGEN.md §5.15, DECISIONS 399: a types-mode parse<T> and parse<Fn>File refuse a value outside its range or length refinement at every position, after the type rules, with one failure, in the bytes every target writes.
func TestTSRangeChecksRun(t *testing.T) {
	runs := typesRuns(t, "range_types", rangeBase, "parseProbe", rangeCases())
	for _, c := range textCases {
		runs = append(runs, rangeRun{"range_types/" + rangeModule, "@" + c.name, rangeCaps, c.text, c.want})
	}
	runRange(t, []string{"range_types"}, runs)
}

// CODEGEN.md §5.15, DECISIONS 399, 400: a bigint at a @ts(bigint) position, a number bound past 2^53 and a bigint map key are checked exactly.
func TestTSRangeBoundsRun(t *testing.T) {
	runRange(t, []string{"range_bounds"}, typesRuns(t, "range_bounds", boundsBase, "parseBounds", boundsCases()))
}

// WIRE.md §5.7, CODEGEN.md §5.15: a keyed list refuses a repeated key at every key type, the token written as the data wire writes it and the earlier row named without the decoder's name; the module type-checks with both compilers.
func TestTSKeyedUnique(t *testing.T) {
	runRange(t, []string{"range_keys"}, typesRuns(t, "range_keys", keysBase, "parseKeys", keysCases()))
}

// WIRE.md §5.8, DECISIONS 399: a map key that is a sized integer is refused outside its width, `integer out of range`, as a value is; a key inside it meets its refinement next.
func TestTSIntKeyWidth(t *testing.T) {
	var runs []rangeRun
	for _, c := range boundsCases() {
		if strings.HasPrefix(c.name, "bytes-") {
			runs = append(runs, typesRuns(t, "range_bounds", boundsBase, "parseBounds", []rangeCase{c})...)
		}
	}
	runRange(t, []string{"range_bounds"}, runs)
}
