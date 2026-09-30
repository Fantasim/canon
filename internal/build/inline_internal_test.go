package build

import (
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	inlineCase   = "testdata/incremental/inline.txtar"
	inlineA      = "a/a.canon"
	inlineSword  = "b/items/sword.canon"
	inlineB      = "b/b.canon"
	inlineW      = "a/w.canon"
	benchMonster = "monster/monster.canon" // the benchmark's record and inline table (benchgen)
	inlineMargin = 2                       // budgets past the cold need, where nothing runs out
)

// inlineStep is one edit of the inline case: a file, the text replaced and its replacement,
// and whether the re-check continues the lineage (the same epoch) or runs a full check.
type inlineStep struct {
	file, old, new string
	kept           bool
}

// inlineSteps edit a record's file and its inline table: rows, lets' values and doc comments
// re-check along the lineage; a changed signature or key set, and an edit before the record,
// which leaves nothing shared, check anew.
var inlineSteps = []inlineStep{
	{inlineA, "hp: 3 }", "hp: 5 }", true},                       // a row
	{inlineA, `name: "Bat"`, `name: "Bats"`, true},              // a second row, the same epoch
	{inlineA, "bonus: Int = 4 +", "bonus: Int = 5 +", true},     // a let's value
	{inlineA, "bonus: Int = 5 +", "bonus: Int = 5 + 0 +", true}, // tokens added before the bounded let
	{inlineA, "Int(0..=50) = 5", "Int(0..=50) = 90", true},      // E3204 quoting its bound, at its new tokens
	{inlineA, "Int(0..=50) = 90", "Int(0..=50) = 5", true},
	{inlineW, "= 5", "= 6", false},                         // a `where`-typed let
	{inlineA, "rat {", `rat @deprecated("gone") {`, false}, // a row's annotation (log-2026-09-30 P15-r (a))
	{inlineA, `rat @deprecated("gone") {`, "rat {", false},
	{inlineA, "size: 4 }", "size: 5 }", true},    // a keyed list's row, not its key
	{inlineA, "level: 2 }", "level: 20 }", true}, // an error in (E3204 against TOP)
	{inlineA, "level: 20 }", "level: 2 }", true},
	{inlineA, "hp: 12 }", `hp: "twelve" }`, true}, // a value of the wrong type
	{inlineA, `hp: "twelve" }`, "hp: 12 }", true},
	{inlineA, "/// The monsters.", "/// Our monsters.", true}, // a let's doc comment
	{inlineSword, "tier: 3", "tier: 4", true},                 // an entry, between them
	{inlineB, "bonus * 2", "bonus * 3", true},                 // an importer's let, after its record
	{inlineA, "hp: 5 }", "hp: 6 }", true},                     // a row again
	{inlineA, "imp {", "ogre {", false},                       // the key set
	{inlineA, "ogre {", "imp {", false},
	{inlineA, "let bonus: Int =", "let bonus: Int(0..=99) =", false},   // a let's type
	{inlineA, "code: 2, size", "code: 3, size", false},                 // a keyed list's key
	{inlineA, "const TOP = 9", "const TOP = 10", false},                // before the record: nothing shared
	{inlineA, "hp: 6 }", "hp: 7 }", true},                              // a row, on the new lineage
	{inlineA, "level: Int(1..=TOP)", "level: Int(1..=TOP + 1)", false}, // the record
	{inlineA, "hp: 7 }", "hp: 3 }", true},                              // a row once more
}

// inlineAt is the inline case with project.canon's budget set to n, none for n = 0.
func inlineAt(t *testing.T, n int) *analyzer {
	t.Helper()
	z := archiveAnalyzer(t, inlineCase)
	if n > 0 {
		z.fs.set(path.Join(archiveRoot, "project.canon"), fmt.Appendf(nil, "project acme {\n  canon: \"0.1\"\n  budget: %d\n}\n", n))
	}
	return z
}

// runInline applies the steps, each warm analysis compared with a cold one; it returns the
// steps whose epoch was not the one expected.
func runInline(t *testing.T, z *analyzer) []string {
	t.Helper()
	cur := map[string]string{}
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	var wrong []string
	for i, st := range inlineSteps {
		name := path.Join(archiveRoot, st.file)
		if cur[name] == "" {
			data, err := z.fs.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			cur[name] = string(data)
		}
		if !strings.Contains(cur[name], st.old) {
			t.Fatalf("step %d: %s holds no %q", i, st.file, st.old)
		}
		cur[name] = strings.Replace(cur[name], st.old, st.new, 1)
		z.fs.set(name, []byte(cur[name]))
		prev := warm.r.epoch
		warm, cold = z.pair(t)
		same(t, fmt.Sprintf("step %d (%q to %q)", i, st.old, st.new), warm, cold)
		if stale := staleObjects(warm.Program()); len(stale) > 0 {
			t.Fatalf("step %d: objects of a replaced file: %v", i, stale)
		}
		if stale := staleSpans(warm); len(stale) > 0 {
			t.Fatalf("step %d: spans in a replaced file: %v", i, stale)
		}
		if (warm.r.epoch == prev) != st.kept {
			wrong = append(wrong, fmt.Sprintf("step %d (%q to %q): kept the epoch %v, want %v", i, st.old, st.new, warm.r.epoch == prev, st.kept))
		}
	}
	return wrong
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: inline table edits equal cold, re-checked iff signatures stay.
func TestIncrementalInlineTable(t *testing.T) {
	if _, cold := inlineAt(t, 0).pair(t); len(cold.Result().List) > 0 {
		t.Fatalf("the case holds findings before any edit: %v", cold.Result().List)
	}
	if wrong := runInline(t, inlineAt(t, 0)); len(wrong) > 0 {
		t.Error(strings.Join(wrong, "\n"))
	}
}

// EVALUATION.md §12.2, DECISIONS 104, IMPLEMENTATION-PLAN §7.6 NFR-02: equal cold at every budget.
func TestIncrementalInlineBudgets(t *testing.T) {
	need := inlineNeed(t)
	t.Logf("a cold analysis needs %d steps", need)
	for n := 1; n <= need+inlineMargin; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			wrong := runInline(t, inlineAt(t, n))
			// Below the need, a shared declaration's fold may run out and report in the old file:
			// the Recheck refuses, a full check gives the same result.
			if n >= need && len(wrong) > 0 {
				t.Error(strings.Join(wrong, "\n"))
			}
		})
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: random edits of the benchmark's monster.canon equal cold.
func TestIncrementalEqualsColdBenchMonster(t *testing.T) {
	if testing.Short() {
		t.Skip("writes a benchmark project with go run")
	}
	dir := filepath.Join(t.TempDir(), "bench")
	cmd := exec.Command("go", "run", "./internal/testkit/cmd/benchgen", "-seed", "1", "-n", benchEntries, "-out", dir)
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("benchgen: %v\n%s", err, out)
	}
	z := &analyzer{fs: newEditFS(project.OS()), dir: filepath.ToSlash(dir), cache: NewCache()}
	monster := []string{path.Join(z.dir, benchMonster)}
	e := newEditor(t, z, monster, monster)
	e.run(t, editSteps)
	if e.lineage == 0 {
		t.Error("no monster edit was re-checked along the lineage")
	}
	warm, _ := z.pair(t)
	if stale := staleSpans(warm); len(stale) > 0 {
		t.Errorf("spans in a replaced file: %v", stale)
	}
}

// staleObjects names each object prog's declarations or Info reach whose file is not one of
// prog's: an object a Recheck should have renewed.
func staleObjects(prog *check.Program) []string {
	current := map[*syntax.File]bool{}
	for _, p := range prog.Packages {
		for _, f := range p.Files {
			current[f] = true
		}
	}
	var out []string
	note := func(o check.Object) {
		if o != nil && o.File() != nil && !current[o.File()] {
			out = append(out, o.Pkg()+"."+o.Name())
		}
	}
	for _, p := range prog.Packages {
		for _, o := range p.Decls {
			note(o)
		}
	}
	info := prog.Info
	for _, m := range []map[*syntax.Ident]check.Object{info.Defs, info.NameUses} {
		for _, o := range m { //canon:unordered the names are sorted below
			note(o)
		}
	}
	for _, o := range info.Uses { //canon:unordered the names are sorted below
		note(o)
	}
	for _, s := range info.Selections { //canon:unordered the names are sorted below
		note(s.Obj)
	}
	for _, c := range info.Calls { //canon:unordered the names are sorted below
		note(c.Obj)
	}
	for o := range info.Broken { //canon:unordered the names are sorted below
		note(o)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// inlineNeed is the least budget a cold analysis of the case does not run out of.
func inlineNeed(t *testing.T) int {
	t.Helper()
	low, high := 1, budgetCeiling
	for low < high {
		mid := (low + high) / 2
		_, cold := inlineAt(t, mid).pair(t)
		if slices.ContainsFunc(cold.Result().List, func(f diag.Finding) bool { return f.Code == diag.E4401.Def().Code }) {
			low = mid + 1
		} else {
			high = mid
		}
	}
	if low == budgetCeiling {
		t.Fatalf("still exhausted at %d steps", budgetCeiling)
	}
	return low
}
