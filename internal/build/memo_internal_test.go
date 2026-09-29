package build

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"path/filepath"
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
)

// memoCorpora are the archives whose projects stages B to D are checked on, cold ≡ recorded ≡ replayed.
var memoCorpora = []string{
	"testdata/*/*.txtar",
	"../verify/testdata/*/*.txtar",
	"../rules/testdata/*/*.txtar",
	"../eval/testdata/*/*.txtar",
}

// replays is how many entries the stage B and stage C memos of z's cache replayed so far.
func (z *analyzer) replays() (verified, checked int) {
	return z.cache.verify.Replayed(), z.cache.rules.Replayed()
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, EVALUATION.md §5, §8.1: one entry edited replays the others.
func TestMemoReplaysUnchangedEntries(t *testing.T) {
	z := archiveAnalyzer(t, memoCase)
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	one := path.Join(archiveRoot, "a/items/one.canon")
	for i := range memoEdits {
		text, err := z.fs.ReadFile(one)
		if err != nil {
			t.Fatal(err)
		}
		z.fs.set(one, fieldNumber.ReplaceAll(text, fmt.Appendf(nil, ": %d", i+1)))
		v0, c0 := z.replays()
		warm, cold = z.pair(t)
		same(t, fmt.Sprintf("edit %d", i), warm, cold)
		v1, c1 := z.replays()
		// seven shares a node with a let (no token), five holds a map (stage B reads its keys' marks),
		// and one is edited: stage B replays two, three, four and six, stage C also five.
		if v1-v0 != 4 || c1-c0 != 5 {
			t.Errorf("edit %d: stage B replayed %d entries, stage C %d; want 4 and 5", i, v1-v0, c1-c0)
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
		one := path.Join(archiveRoot, "a/items/one.canon")
		entry, err := z.fs.ReadFile(one)
		if err != nil {
			t.Fatal(err)
		}
		z.fs.set(one, fieldNumber.ReplaceAll(entry, []byte(": 7")))
		v0, c0 := z.replays()
		warm, cold = z.pair(t)
		same(t, "edited", warm, cold)
		if v1, c1 := z.replays(); v1-v0 != 4 || c1 != c0 {
			t.Errorf("read before %t: stage B replayed %d entries, stage C %d; want 4 and none", before, v1-v0, c1-c0)
		}
	}
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
			keyedHaveIdentities(t, file, cold)
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
	unlike := 0
	for n := 1; n <= low; n++ {
		with, without := memoBudgeted(t, n), memoBudgeted(t, n)
		without.cache.verify, without.cache.rules = nil, nil
		unlike += budgetSteps(t, n, with, without)
	}
	t.Logf("%d analyses without the memos differ from cold (a stage-E fold along a Recheck lineage)", unlike)
}

// budgetSteps analyzes with and without along the same edits of one entry; each analysis with
// the memos is the one without, and cold's when that one is. It returns how many are not cold's.
func budgetSteps(t *testing.T, n int, with, without *analyzer) int {
	t.Helper()
	one := path.Join(archiveRoot, "a/items/one.canon")
	unlike := 0
	for i := range budgetEdits + 1 {
		if i > 0 {
			text, err := with.fs.ReadFile(one)
			if err != nil {
				t.Fatal(err)
			}
			edited := fieldNumber.ReplaceAll(text, fmt.Appendf(nil, ": %d", i))
			with.fs.set(one, edited)
			without.fs.set(one, edited)
		}
		warm, cold := with.pair(t)
		plain, _ := without.pair(t)
		name := fmt.Sprintf("budget %d, step %d", n, i)
		w, p, c := dumpAnalysis(t, warm), dumpAnalysis(t, plain), dumpAnalysis(t, cold)
		switch {
		case w != p:
			same(t, name+" (memos)", warm, plain)
		case p != c:
			unlike++
		case w != c:
			same(t, name, warm, cold)
		}
	}
	return unlike
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
