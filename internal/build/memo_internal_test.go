package build

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"golang.org/x/tools/txtar"
)

const (
	memoCase       = "testdata/incremental/memo.txtar"
	memoEdits      = 8
	corpusEdits    = 2
	memoProject    = "project.canon"
	memoBudgetLine = "  canon: \"0.1\"\n"
	memoPkgFile    = "a/a.canon"
	memoSource     = ".canon"
	// eval's harness reads a budget from this file; a build would run such a case at 10⁸ steps
	memoHarnessBudget = "budget"
	memoDefault       = "project demo {\n  canon: \"0.1\"\n}\n"
	memoTable         = "/// Items.\nlet items"
	memoReader        = "/// The items again.\nlet again: [Item] = items.values()\n\n" // holds the table's entries
	memoFirst         = "a/items/one.canon"
	selectionsCase    = "testdata/incremental/selections.txtar"
	selectionsEdited  = "a/items/two.canon"
	selectionsZ       = "b/bits/z.canon"
	selectionA        = "a"
	selectionB        = "b"
	marksCase         = "testdata/incremental/marks.txtar"
	marksLast         = "a/d/last.canon"
	marksWeight       = 300
	pinnedLow         = 4
	pinnedHigh        = 7
	digitsMark        = "N"
)

// pinnedLines are the lines, trimmed and each number digitsMark, that the pinned divergence adds,
// removes or changes: stage E's E4401 as text, as JSON, in the view model, and the summaries.
var pinnedLines = []string{
	"", "{", "},",
	`"code": "EN",`, `"col": N,`, `"endCol": N,`, `"endLine": N,`, `"line": N,`,
	`"file": "a/a.canon",`, `"package": "a",`, `"severity": "error",`,
	`"message": "evaluation budget of N steps exhausted\nheaviest: Item (N steps)"`,
	`"message": "evaluation budget of N steps exhausted\nheaviest: Part (N steps)"`,
	"error[EN]  a/a.canon:N:N", "evaluation budget of N steps exhausted",
	"heaviest: Item (N steps)", "heaviest: Part (N steps)",
	"N error, N warnings in N package (…)", "N errors, N warnings in N package (…)",
	`{"summary":{"errors":N,"warnings":N,"packages":N,"ms":N}}`,
	`{"severity":"error","code":"EN","file":"a/a.canon","line":N,"col":N,"endLine":N,"endCol":N,"package":"a","message":"evaluation budget of N steps exhausted\nheaviest: Item (N steps)"}`,
	`{"severity":"error","code":"EN","file":"a/a.canon","line":N,"col":N,"endLine":N,"endCol":N,"package":"a","message":"evaluation budget of N steps exhausted\nheaviest: Part (N steps)"}`,
}

// digits are the numbers pinnedLines abstract.
var digits = regexp.MustCompile(`[0-9]+`)

// memoCorpora are the archives whose projects stages B to D are checked on, cold ≡ recorded ≡ replayed.
var memoCorpora = []string{
	"testdata/*/*.txtar",
	"../verify/testdata/*/*.txtar",
	"../rules/testdata/*/*.txtar",
	"../eval/testdata/*/*.txtar",
}

// replaysOf is how many entries a's stages B and C replayed.
func replaysOf(a *Analysis) (verified, checked int) {
	return a.r.host.verifier.Replayed(), a.r.runner.Replayed()
}

// edit sets every number of the file at name, under archiveRoot, to n.
func (z *analyzer) edit(t *testing.T, name string, n int) {
	t.Helper()
	abs := path.Join(archiveRoot, name)
	text, err := z.fs.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	z.fs.set(abs, fieldNumber.ReplaceAll(text, fmt.Appendf(nil, ": %d", n)))
}

// replaysAfterEdit edits name, analyzes z warm and cold, and is what the warm analysis replayed.
func (z *analyzer) replaysAfterEdit(t *testing.T, name string, n int) (verified, checked int) {
	t.Helper()
	z.edit(t, name, n)
	warm, cold := z.pair(t)
	same(t, fmt.Sprintf("%s set to %d", name, n), warm, cold)
	return replaysOf(warm)
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, EVALUATION.md §5, §8.1: one entry edited replays the others.
func TestMemoReplaysUnchangedEntries(t *testing.T) {
	z := archiveAnalyzer(t, memoCase)
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	for i := range memoEdits {
		// seven shares a node with a let (no token), five holds a map (stage B reads its keys' marks),
		// and one is edited: stage B replays two, three, four and six, stage C also five.
		if v, c := z.replaysAfterEdit(t, memoFirst, i+1); v != 4 || c != 5 {
			t.Errorf("edit %d: stage B replayed %d entries, stage C %d; want 4 and 5", i, v, c)
		}
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, EVALUATION.md §8.1: a value holding a part of the table keeps stage C traversing it.
func TestMemoSharedTable(t *testing.T) {
	for _, before := range []bool{false, true} {
		z := archiveAnalyzer(t, memoCase)
		abs := path.Join(archiveRoot, memoPkgFile)
		text, err := z.fs.ReadFile(abs)
		if err != nil {
			t.Fatal(err)
		}
		edited := append(slices.Clone(text), memoReader...)
		if before {
			edited = bytes.Replace(text, []byte(memoTable), []byte(memoReader+memoTable), 1)
		}
		z.fs.set(abs, edited)
		warm, cold := z.pair(t)
		same(t, "first", warm, cold)
		if v, c := z.replaysAfterEdit(t, memoFirst, memoEdits); v != 4 || c != 0 {
			t.Errorf("read before %t: stage B replayed %d entries, stage C %d; want 4 and none", before, v, c)
		}
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02 (log-2026-09-29 M4 P3-r, P12-r): two selections alternating keep their replays.
func TestMemoTwoSelections(t *testing.T) {
	z := archiveAnalyzer(t, selectionsCase)
	for i := range memoEdits {
		z.edit(t, selectionsEdited, i+1)
		a := z.selectedPair(t, selectionA)
		z.edit(t, selectionsZ, i+1)
		b := z.selectedPair(t, selectionB)
		if i == 0 {
			continue
		}
		// a: four, first in path order, finds b.lim not forced yet: ReplayChecks refuses; two is edited
		if v, c := replaysOf(a); v != 3 || c != 2 {
			t.Errorf("round %d, a: stage B replayed %d entries, stage C %d; want 3 and 2", i, v, c)
		}
		if v, c := replaysOf(b); v != 2 || c != 2 {
			t.Errorf("round %d, b: stage B replayed %d entries, stage C %d; want 2 and 2", i, v, c)
		}
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02 (log-2026-09-29 M4 P12-r): a forgotten epoch replays nothing, in any stage.
func TestMemoForgottenEpoch(t *testing.T) {
	z := archiveAnalyzer(t, memoCase)
	warm, _ := z.pair(t)
	z.cache.memo.Forget(warm.r.epoch)
	if v, c := z.replaysAfterEdit(t, memoFirst, 1); v != 0 || c != 0 {
		t.Errorf("forgotten: stage B replayed %d entries, stage C %d", v, c)
	}
	z = archiveAnalyzer(t, memoCase)
	z.pair(t)
	old := z.cache.gen
	old.mu.Lock()
	old.total, old.live = compactFloor+1, 0
	old.mu.Unlock()
	if v, c := z.replaysAfterEdit(t, memoFirst, 2); v != 0 || c != 0 || z.cache.gen == old {
		t.Errorf("compacted: stage B replayed %d entries, stage C %d", v, c)
	}
	if v, c := z.replaysAfterEdit(t, memoFirst, 3); v != 4 || c != 5 {
		t.Errorf("after compaction: stage B replayed %d entries, stage C %d; want 4 and 5", v, c)
	}
}

// EVALUATION.md §7.3, §8.1, IMPLEMENTATION-PLAN §7.6: check runs marking values, stage C replayed around them.
func TestMemoMarkingChecks(t *testing.T) {
	z := archiveAnalyzer(t, marksCase)
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	for i := range memoEdits {
		// shared holds s, a let's (no token); own's check marks its own value (not traced)
		if v, c := z.replaysAfterEdit(t, marksLast, marksWeight+i); v != 2 || c != 1 {
			t.Errorf("edit %d: stage B replayed %d entries, stage C %d; want 2 and 1", i, v, c)
		}
	}
}

// selectedPair analyzes the package selected warm and cold, the same, and is the warm analysis.
func (z *analyzer) selectedPair(t *testing.T, selected string) *Analysis {
	t.Helper()
	ctx := context.Background()
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	warm, err := p.WithCache(z.cache).Analyze(ctx, []string{selected})
	if err != nil {
		t.Fatal(err)
	}
	cold, err := p.Analyze(ctx, []string{selected})
	if err != nil {
		t.Fatal(err)
	}
	same(t, selected, warm, cold)
	return warm
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: every corpus cold ≡ recorded ≡ replayed, then edited.
func TestMemoEqualsColdCorpora(t *testing.T) {
	for _, file := range corpusArchives(t) {
		t.Run(strings.TrimSuffix(filepath.ToSlash(file), ".txtar"), func(t *testing.T) {
			z := archiveAnalyzer(t, file)
			m := z.fs.base.(roFS)
			if at := strings.TrimPrefix(path.Join(archiveRoot, memoProject), "/"); m[at] == nil {
				m[at] = srcFile(memoDefault) // a rules case is one package and no project file
			}
			warm, cold, ok := z.tryPair(t)
			if !ok {
				return
			}
			same(t, "recorded", warm, cold)
			warm, cold, _ = z.tryPair(t)
			same(t, "replayed", warm, cold)
			files := numbered(warm)
			if len(files) == 0 {
				return
			}
			newEditor(t, z, files, files).run(t, corpusEdits)
		})
	}
}

// EVALUATION.md §12.2, IMPLEMENTATION-PLAN §7.6 NFR-02: replays charge and stop as runs do.
func TestMemoBudgetCuts(t *testing.T) {
	low, high := 1, budgetCeiling
	for low < high {
		mid := (low + high) / 2
		if _, cold := memoBudgeted(t, mid).pair(t); exhaustedIn(cold) {
			low = mid + 1
		} else {
			high = mid
		}
	}
	t.Logf("a cold analysis needs %d steps", low)
	for n := 1; n <= low; n++ {
		budgetSteps(t, n, memoBudgeted(t, n))
	}
}

// budgetSteps analyzes z along edits of one entry, each warm analysis as cold but for the one
// divergence pinned (pinnedBudget).
func budgetSteps(t *testing.T, n int, z *analyzer) {
	t.Helper()
	one := path.Join(archiveRoot, memoFirst)
	for i := range budgetEdits + 1 {
		if i > 0 {
			text, err := z.fs.ReadFile(one)
			if err != nil {
				t.Fatal(err)
			}
			z.fs.set(one, fieldNumber.ReplaceAll(text, fmt.Appendf(nil, ": %d", i)))
		}
		warm, cold := z.pair(t)
		w, c := dumpAnalysis(t, warm), dumpAnalysis(t, cold)
		if w != c && !pinnedBudget(n, w, c) {
			same(t, fmt.Sprintf("budget %d, step %d", n, i), warm, cold)
		}
	}
}

// pinnedBudget reports the one known divergence of a warm analysis from cold: along a Recheck
// lineage, stage E's fold runs out of steps elsewhere, memo.txtar at budgets 4 to 7 (log-2026-09-29
// M4 P12-r; unit B1 removes it, and this pin with it).
func pinnedBudget(n int, warm, cold string) bool {
	if n < pinnedLow || n > pinnedHigh {
		return false
	}
	for _, line := range unmatched(warm, cold) {
		if !slices.Contains(pinnedLines, digits.ReplaceAllString(strings.TrimSpace(line), digitsMark)) {
			return false
		}
	}
	return true
}

// unmatched is the lines of each of a and b the other lacks, counted as a multiset.
func unmatched(a, b string) []string {
	left := map[string]int{}
	for _, l := range strings.Split(b, "\n") {
		left[l]++
	}
	var out []string
	for _, l := range strings.Split(a, "\n") {
		if left[l] > 0 {
			left[l]--
		} else {
			out = append(out, l)
		}
	}
	for _, l := range slices.Sorted(maps.Keys(left)) {
		for range left[l] {
			out = append(out, l)
		}
	}
	return out
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, API.md S7-S8: snapshots share the stage B and C memos.
func TestMemoConcurrentSnapshots(t *testing.T) {
	base := archiveAnalyzer(t, memoCase)
	two := path.Join(archiveRoot, "a/items/two.canon")
	text, err := base.fs.ReadFile(two)
	if err != nil {
		t.Fatal(err)
	}
	analyzeTwice(t, base) // every entry recorded
	var wg sync.WaitGroup
	dumps := make([][2]string, parallelSnapshots)
	for i := range parallelSnapshots {
		z := &analyzer{fs: newEditFS(base.fs.base), dir: base.dir, cache: base.cache}
		z.fs.set(two, fieldNumber.ReplaceAll(text, fmt.Appendf(nil, ": %d", i%3)))
		wg.Add(1)
		go func() {
			defer wg.Done()
			dumps[i] = analyzeTwice(t, z)
		}()
	}
	wg.Wait()
	for i, d := range dumps {
		if d[0] != d[1] {
			t.Errorf("snapshot %d: warm differs from cold", i)
		}
	}
}

// memoBudgeted is the memo case with project.canon's budget set to n (EVALUATION.md §12.2).
func memoBudgeted(t *testing.T, n int) *analyzer {
	t.Helper()
	z := archiveAnalyzer(t, memoCase)
	abs := path.Join(archiveRoot, memoProject)
	text, err := z.fs.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	z.fs.set(abs, bytes.Replace(text, []byte(memoBudgetLine), fmt.Appendf(nil, "%s  budget: %d\n", memoBudgetLine, n), 1))
	return z
}

// exhaustedIn reports an analysis that ran out of steps (E4401).
func exhaustedIn(a *Analysis) bool {
	return slices.ContainsFunc(a.Result().List, func(f diag.Finding) bool { return f.Code == diag.E4401.Def().Code })
}

// corpusArchives is every archive of memoCorpora holding a source, in path order.
func corpusArchives(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, glob := range memoCorpora {
		files, err := filepath.Glob(glob)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			a, err := txtar.ParseFile(f)
			if err != nil {
				t.Fatal(err)
			}
			source := slices.ContainsFunc(a.Files, func(af txtar.File) bool { return path.Ext(af.Name) == memoSource })
			budgeted := slices.ContainsFunc(a.Files, func(af txtar.File) bool { return af.Name == memoHarnessBudget })
			if source && !budgeted {
				out = append(out, f)
			}
		}
	}
	slices.Sort(out)
	return out
}

// tryPair is pair for a project that may fail to analyze: false when it does, warm and cold
// failing alike.
func (z *analyzer) tryPair(t *testing.T) (warm, cold *Analysis, ok bool) {
	t.Helper()
	ctx := context.Background()
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		return nil, nil, false
	}
	warm, werr := p.WithCache(z.cache).Analyze(ctx, nil)
	cold, cerr := p.Analyze(ctx, nil)
	if (werr == nil) != (cerr == nil) || werr != nil && werr.Error() != cerr.Error() {
		t.Fatalf("warm error %v, cold error %v", werr, cerr)
	}
	return warm, cold, cerr == nil
}

// numbered is every source of a's program whose fields a number sets, by absolute name.
func numbered(a *Analysis) []string {
	var out []string
	for _, cp := range a.Program().Packages {
		for _, f := range cp.Files {
			if fieldNumber.Match(f.Src.Content) && !strings.HasSuffix(f.Src.Path, memoProject) {
				out = append(out, f.Src.Abs)
			}
		}
	}
	slices.Sort(out)
	return out
}
