package eval_test

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	entriesCase  = "testdata/memo/entries.txtar"
	cfgFile      = "resource/cfg.json"
	editedEntry  = "a/items/plain.canon"
	parallelRuns = 8
)

// memoAt is a memo used in an epoch.
type memoAt struct {
	memo  *eval.Memo
	epoch uint64
}

// replays is the entries an evaluation replayed and kept.
type replays struct {
	hits, stored int
}

// against evaluates m cold and with at's memo, and reports a difference or unexpected counts.
func against(t *testing.T, name string, m *memoed, at memoAt, want replays) outcome {
	t.Helper()
	cold := m.evaluate(t, nil, 0)
	got := m.evaluate(t, at.memo, at.epoch)
	same(t, name, got, cold)
	if got.hits != want.hits || got.stored != want.stored {
		t.Errorf("%s: replayed %d and kept %d entries, want %d and %d", name, got.hits, got.stored, want.hits, want.stored)
	}
	return got
}

// IMPLEMENTATION-PLAN §7.6 (NFR-02), EVALUATION.md §3.1, §13: a read value changed misses, the rest replays.
func TestMemoReadChanges(t *testing.T) {
	m, a := archived(t, "testdata/memo/reads.txtar")
	var cfg []byte
	m.load = servedJSON(t, m.p, func(name string) []byte {
		if name == cfgFile {
			return cfg
		}
		return archiveFile(a, name)
	})
	at := memoAt{memo: eval.NewMemo(), epoch: 1}
	for i, run := range []struct {
		cfg  string
		want replays
	}{
		{`{"bonus": 4}`, replays{0, 2}}, {`{"bonus": 4}`, replays{2, 0}},
		{`{"bonus": 5}`, replays{1, 1}}, {`{"bonus": 5}`, replays{2, 0}}, {`{"bonus": 4}`, replays{1, 1}},
	} {
		cfg = []byte(run.cfg)
		against(t, fmt.Sprint("run ", i), m, at, run.want)
	}
}

// IMPLEMENTATION-PLAN §7.6, EVALUATION.md §9.1, §9.3 (LAY-02): each set of active layers replays its own entries.
func TestMemoLayers(t *testing.T) {
	m, _ := archived(t, "testdata/memo/layers.txtar")
	at := memoAt{memo: eval.NewMemo(), epoch: 1}
	for i, run := range []struct {
		layers []string
		want   replays
	}{
		{nil, replays{0, 2}}, {[]string{"dev"}, replays{0, 1}}, {nil, replays{2, 0}}, {[]string{"dev"}, replays{1, 0}},
	} {
		m.opt.Layers = run.layers
		against(t, fmt.Sprint("run ", i, run.layers), m, at, run.want)
	}
}

// EVALUATION.md §3.3, DECISIONS 195: an entry whose calls pass 10,000 frames where it is forced is evaluated (E4402).
func TestMemoDepth(t *testing.T) {
	m, _ := archived(t, "testdata/memo/depth.txtar")
	at := memoAt{memo: eval.NewMemo(), epoch: 1}
	m.first = []eval.Root{{Pkg: "a", Name: "items"}}
	against(t, "items first", m, at, replays{0, 2})
	m.first = nil
	code := "[" + string(diag.E4402.Def().Code) + "]"
	if got := against(t, "items deep", m, at, replays{1, 0}); !strings.Contains(got.findings, code) {
		t.Errorf("items deep: no %s:\n%s", code, got.findings)
	}
}

// EVALUATION.md §12.2 (EVL-03): at every budget, entries replayed stop where a cold evaluation does.
func TestMemoBudgetCuts(t *testing.T) {
	for _, path := range []string{entriesCase, "testdata/memo/stop.txtar"} {
		m, _ := archived(t, path)
		memo := eval.NewMemo()
		full := m.evaluate(t, memo, 1)
		for budget := int64(1); budget <= full.steps; budget++ {
			m.opt.Budget = budget
			cold := m.evaluate(t, nil, 0)
			same(t, fmt.Sprint(path, " budget ", budget), m.evaluate(t, memo, 1), cold)
		}
	}
}

// EVALUATION.md §7.2 (EVL-05): a replay stopped by a value read now poisoned reports the findings before it.
func TestMemoPoisonedRead(t *testing.T) {
	m, a := archived(t, "testdata/memo/poisonread.txtar")
	cfg := archiveFile(a, cfgFile)
	m.load = servedJSON(t, m.p, func(name string) []byte {
		if name == cfgFile {
			return cfg
		}
		return archiveFile(a, name)
	})
	at := memoAt{memo: eval.NewMemo(), epoch: 1}
	against(t, "decoded", m, at, replays{0, 1})
	cfg = []byte(`{"bonus": "x"}`)
	code := "[" + string(diag.E3204.Def().Code) + "]"
	if got := against(t, "poisoned", m, at, replays{0, 0}); !strings.Contains(got.findings, code) {
		t.Errorf("poisoned: no %s:\n%s", code, got.findings)
	}
}

// IMPLEMENTATION-PLAN §7.6, DOCTRINE §5: evaluators sharing one memo concurrently each give the cold outcome.
func TestMemoConcurrent(t *testing.T) {
	for _, path := range []string{entriesCase, "testdata/memo/foreign.txtar", "testdata/memo/stop.txtar"} {
		concurrently(t, path)
	}
}

// concurrently evaluates the case at path in parallel evaluators sharing one memo, twice.
func concurrently(t *testing.T, path string) {
	m, _ := archived(t, path)
	cold := m.evaluate(t, nil, 0)
	memo := eval.NewMemo()
	for wave := range 2 {
		outs := make([]outcome, parallelRuns)
		var wg sync.WaitGroup
		for i := range outs {
			wg.Go(func() { outs[i] = m.evaluate(t, memo, 1) })
		}
		wg.Wait()
		for i, out := range outs {
			same(t, fmt.Sprint(path, " wave ", wave, " evaluator ", i), out, cold)
		}
	}
}

// IMPLEMENTATION-PLAN §7.6: a new epoch, or a program checked again after an entry file changed, replays nothing.
func TestMemoEpoch(t *testing.T) {
	m, a := archived(t, entriesCase)
	memo := eval.NewMemo()
	against(t, "epoch 1", m, memoAt{memo: memo, epoch: 1}, replays{0, 6})
	against(t, "epoch 2", m, memoAt{memo: memo, epoch: 2}, replays{0, 6})
	files := slices.Clone(m.p.files)
	at := slices.IndexFunc(files, func(f *syntax.File) bool { return f.Src.Path == editedEntry })
	edited := strings.Replace(string(archiveFile(a, editedEntry)), `"Plain"`, `"Plainer"`, 1)
	src, err := m.p.fs.Add(editedEntry, "/"+editedEntry, []byte(edited))
	if at < 0 || err != nil {
		t.Fatalf("%s: %d, %v", editedEntry, at, err)
	}
	files[at] = syntax.Parse(src, syntax.FileSource, m.p.parse)
	next := checkOnce(t, &program{fs: m.p.fs, files: files, parse: m.p.parse}, m.opt)
	next.load = m.load
	against(t, "edited", next, memoAt{memo: memo, epoch: 3}, replays{0, 6})
	against(t, "edited again", next, memoAt{memo: memo, epoch: 3}, replays{6, 0})
}

// IMPLEMENTATION-PLAN §7.6 (log-2026-09-29 M4 U11): the same files checked again by check.Check in the same epoch, a misuse, never give a wrong outcome.
func TestMemoSameEpochRecheck(t *testing.T) {
	m, _ := archived(t, entriesCase)
	memo := memoAt{memo: eval.NewMemo(), epoch: 1}
	against(t, "first check", m, memo, replays{0, 6})
	again := checkOnce(t, m.p, m.opt)
	again.load = m.load
	cold := again.evaluate(t, nil, 0)
	same(t, "checked again", again.evaluate(t, memo.memo, memo.epoch), cold)
}

// IMPLEMENTATION-PLAN §7.6: one entry edited, the others replayed in one epoch, needs check.Session.
func TestMemoRecheckLineage(t *testing.T) {
	t.Skip("incremental ≡ cold across check.Session.Recheck and the memo is U8's property test (log-2026-09-29 M4 U11)")
}
