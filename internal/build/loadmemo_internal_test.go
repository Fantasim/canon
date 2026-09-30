package build

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
)

const (
	loadFormsCase = "testdata/incremental/loadforms.txtar"
	loadEntryFile = "a/e1.canon"
	loadItemFile  = "resource/items/i1.json"
	loadSteps     = 4
	budgetFormat  = "project acme {\n  canon: \"0.1\"\n  budget: %d\n\n  roots {\n    resource: \"resource\"\n  }\n}\n"
)

// pairSel is pair for the packages sel.
func (z *analyzer) pairSel(t *testing.T, sel []string) (warm, cold *Analysis) {
	t.Helper()
	ctx := context.Background()
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	if warm, err = p.WithCache(z.cache).Analyze(ctx, sel); err != nil {
		t.Fatal(err)
	}
	if cold, err = p.Analyze(ctx, sel); err != nil {
		t.Fatal(err)
	}
	return warm, cold
}

// grown is the files the cache generation's set gained while f ran.
func grown(z *analyzer, f func()) source.FileID {
	g := z.cache.gen
	before := g.last()
	f()
	return g.last() - before
}

// loadStep changes files of the load forms case on disk.
type loadStep struct {
	name string
	do   func(m roFS)
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, WIRE.md §6.1, §6.5-§6.8, API.md S3: every load form's files changed, warm as cold.
func TestIncrementalLoadForms(t *testing.T) {
	z := archiveAnalyzer(t, loadFormsCase)
	m := z.fs.base.(roFS)
	file := func(name string) string { return strings.TrimPrefix(path.Join(archiveRoot, name), "/") }
	put := func(name, data string) func(roFS) { return func(m roFS) { m[file(name)] = srcFile(data) } }
	edit := func(name, old, new string) func(roFS) {
		return func(m roFS) { m[file(name)] = srcFile(strings.Replace(string(m[file(name)].Data), old, new, 1)) }
	}
	nop := func(roFS) {}
	item := string(m[file(loadItemFile)].Data)
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	for _, st := range []loadStep{
		{"unchanged", nop},
		{"json edited", edit(loadItemFile, `"cost": 5`, `"cost": 6`)},
		{"json added", put("resource/items/i3.json", `{"sku": "i3", "code": "C_TWO", "cost": 1}`)},
		{"entry edited", edit(loadEntryFile, "cost: 4", "cost: 7")},
		{"json removed", func(m roFS) { delete(m, file("resource/items/i2.json")) }},
		{"json broken", edit(loadItemFile, `"cost": 6`, `"cost": "x"`)},
		{"json put back", put(loadItemFile, item)},
		{"csv edited", put("resource/rows.csv", "name,size\nx,2\ny,3\n")},
		{"text edited", put("resource/note.txt", "hello\nworld\n")},
		{"header edited", put("resource/codes.h", "#define C_ONE 1\n#define C_TWO 2\n")},
		{"cfg edited", put("resource/cfg.json", `{"level": 9}`)},
		{"ref broken", put("resource/items/i3.json", `{"sku": "i3", "code": "C_NONE"}`)},
		{"unchanged again", nop},
	} {
		st.do(m)
		n := grown(z, func() { warm, cold = z.pair(t) })
		same(t, st.name, warm, cold)
		unchangedReplays(t, st.name, warm, n)
	}
}

// unchangedReplays fails an unchanged step whose loads grew the file set or were not replayed.
func unchangedReplays(t *testing.T, name string, warm *Analysis, grew source.FileID) {
	t.Helper()
	if n := warm.r.ev.LoadsReplayed(); strings.HasPrefix(name, "unchanged") && (grew != 0 || n == 0) {
		t.Errorf("%s: the file set gained %d files, %d loads replayed", name, grew, n)
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, WIRE.md §6: random edits of the files every example's loads read, warm as cold.
func TestIncrementalEqualsColdExampleLoads(t *testing.T) {
	if testing.Short() {
		t.Skip("every example, analyzed twice per edit")
	}
	z := examplesAnalyzer(t)
	warm, _ := z.pair(t)
	var files []string
	for _, cp := range warm.Program().Packages {
		for _, r := range warm.Reads(cp.Path) {
			data, err := z.fs.ReadFile(r.Abs)
			if !r.Dir && strings.HasPrefix(r.Display, rootMark) && err == nil && fieldNumber.Match(data) && !slices.Contains(files, r.Abs) {
				files = append(files, r.Abs)
			}
		}
	}
	if len(files) == 0 {
		t.Fatal("no loaded file with a number to edit")
	}
	slices.Sort(files)
	t.Logf("%d loaded files, edited at random", len(files))
	e := newEditor(t, z, files, files)
	e.run(t, editSteps)
}

// EVALUATION.md §12.2, IMPLEMENTATION-PLAN §7.6 NFR-02: at every budget, replayed loads stop as cold ones.
func TestIncrementalLoadBudgetCuts(t *testing.T) {
	low, high := 1, budgetCeiling
	for low < high { // the least budget the cold analysis does not exhaust
		mid := (low + high) / 2
		if _, cold := loadsBudgeted(t, mid).pair(t); slices.ContainsFunc(cold.Result().List, isE4401) {
			low = mid + 1
		} else {
			high = mid
		}
	}
	files := []string{path.Join(archiveRoot, loadEntryFile), path.Join(archiveRoot, loadItemFile)}
	for i := range budgetSamples {
		e := newEditor(t, loadsBudgeted(t, 1+i*low/(budgetSamples-1)), files, files[:1])
		e.run(t, loadSteps)
	}
}

// loadsBudgeted is the load forms case with project.canon's budget set to n (EVALUATION.md §12.2).
func loadsBudgeted(t *testing.T, n int) *analyzer {
	t.Helper()
	z := archiveAnalyzer(t, loadFormsCase)
	z.fs.set(path.Join(archiveRoot, "project.canon"), fmt.Appendf(nil, budgetFormat, n))
	return z
}

func isE4401(f diag.Finding) bool { return f.Code == diag.E4401.Def().Code }

// IMPLEMENTATION-PLAN §7.6 NFR-02: two selections in turn each keep their own lineage and epoch, warm as cold.
func TestCacheLineagePerSelection(t *testing.T) {
	z := archiveAnalyzer(t, loadFormsCase)
	entry := path.Join(archiveRoot, loadEntryFile)
	text, err := z.fs.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	epochs := map[string]uint64{}
	for i := range 2 * loadSteps {
		sel := []string{"a"}
		if i%2 == 1 {
			sel = nil
		}
		z.fs.set(entry, []byte(strings.Replace(string(text), "cost: 4", fmt.Sprintf("cost: %d", i), 1)))
		warm, cold := z.pairSel(t, sel)
		name := fmt.Sprint(sel)
		same(t, fmt.Sprint("run ", i, " ", name), warm, cold)
		if prev, ok := epochs[name]; ok && warm.r.epoch != prev {
			t.Errorf("run %d %s: epoch %d, the lineage's was %d", i, name, warm.r.epoch, prev)
		}
		epochs[name] = warm.r.epoch
	}
	if len(epochs) != 2 || epochs["[a]"] == epochs["[]"] || len(z.cache.gen.heads) != 2 {
		t.Errorf("epochs %v, %d lineages: want two lineages, each its own epoch", epochs, len(z.cache.gen.heads))
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02 (log-2026-09-29 M4 P3-r): a compaction forgets the old generation's epochs for good.
func TestCacheCompactionForgetsEpochs(t *testing.T) {
	z := archiveAnalyzer(t, loadFormsCase)
	before, _ := z.pair(t)
	old, epoch := z.cache.gen, before.r.epoch
	if !z.cache.memo.Kept(epoch) {
		t.Fatalf("epoch %d: no store before the compaction", epoch)
	}
	old.mu.Lock()
	old.total, old.live = compactFloor+1, 0
	old.mu.Unlock()
	for i := range loadSteps {
		warm, cold := z.pair(t)
		same(t, fmt.Sprint("after the compaction, run ", i), warm, cold)
		if z.cache.gen == old || !z.cache.memo.Kept(warm.r.epoch) {
			t.Fatalf("run %d: the set was not replaced, or the new lineage has no store", i)
		}
	}
	ev := eval.New(before.Program(), nil, check.Bags{}, eval.Options{})
	ev.UseMemo(z.cache.memo, epoch) // a late use of the old lineage's epoch
	if z.cache.memo.Kept(epoch) {
		t.Errorf("epoch %d: the old generation's store is kept, or made again", epoch)
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02 (log-2026-09-29 M4 P3-r): a full check in a generation compacted meanwhile keeps no store.
func TestCacheCheckInDeadGeneration(t *testing.T) {
	ctx := context.Background()
	z := archiveAnalyzer(t, loadFormsCase)
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.WithCache(z.cache).prepare(ctx, nil) // begun on the first generation, not checked yet
	if err != nil {
		t.Fatal(err)
	}
	old := z.cache.gen
	old.mu.Lock()
	old.total, old.live = compactFloor+1, 0
	old.mu.Unlock()
	z.pair(t) // compacts
	r.causes = true
	if err := r.analyze(ctx); err != nil {
		t.Fatal(err)
	}
	if r.s.gen != old || z.cache.gen == old || r.epoch == 0 || z.cache.memo.Kept(r.epoch) {
		t.Fatalf("epoch %d checked in the dead generation: want its store forgotten", r.epoch)
	}
	_, cold := z.pair(t)
	same(t, "checked in the dead generation", r.analysis(), cold)
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: lineageCap lineages, the least recently checked dropped, no stale Recheck kept.
func TestCacheLineagesBounded(t *testing.T) {
	g := newCacheGen()
	heads := make([]*head, lineageCap+1)
	var gone []uint64
	for i := range heads {
		heads[i] = &head{key: lineageKey{pkgs: fmt.Sprint(i)}, epoch: uint64(i + 1)}
		gone = g.advance(nil, heads[i])
	}
	if len(g.heads) != lineageCap || g.lineage(heads[0].key) != nil || g.lineage(heads[lineageCap].key) != heads[lineageCap] {
		t.Fatalf("%d lineages kept: want %d, the first dropped", len(g.heads), lineageCap)
	}
	next := &head{key: heads[1].key, epoch: heads[1].epoch}
	rechecked := g.advance(heads[1], next)
	stale := &head{key: heads[1].key, epoch: heads[1].epoch}
	g.advance(heads[1], stale)
	if g.lineage(heads[1].key) != next || g.heads[0] != next || len(rechecked) != 0 {
		t.Error("a Recheck from a replaced head was kept, or the rechecked lineage is not the latest")
	}
	full := g.advance(nil, &head{key: heads[1].key, epoch: uint64(len(heads) + 1)})
	if !slices.Equal(gone, []uint64{heads[0].epoch}) || !slices.Equal(full, []uint64{next.epoch}) {
		t.Errorf("epochs no head keeps: %v evicted, %v replaced", gone, full)
	}
}
