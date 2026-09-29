package eval_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	loadsCase = "testdata/memo/loads.txtar"
	rowsFile  = "resource/rows.json"
	evsFile   = "resource/evs.json"
)

// loadCases are the programs whose loads a memo replays: defaults, dependent fields, layers, provenance.
var loadCases = []string{loadsCase, "testdata/memo/reads.txtar", "testdata/prov/loaded.txtar", "testdata/prov/origins.txtar", "testdata/prov/layered.txtar"}

// loadMemoHost is the fixture host as a LoadMemo: a load's inputs are the bytes it was served.
type loadMemoHost struct {
	*host
	files func(name string) []byte
}

func (h *loadMemoHost) LoadRecorded(ctx context.Context, e *syntax.LoadExpr, t types.Type) (value.Value, bool, eval.LoadInputs) {
	v, ok := h.Load(ctx, e, t)
	return v, ok, string(h.files(loadName(e)))
}

func (h *loadMemoHost) LoadReplay(_ context.Context, e *syntax.LoadExpr, in eval.LoadInputs) (func(), bool) {
	return func() {}, in == eval.LoadInputs(string(h.files(loadName(e))))
}

// loadName is the file a fixture load names, its root mark cut as servedJSON cuts it.
func loadName(e *syntax.LoadExpr) string {
	lit, ok := e.Args[0].Value.(*syntax.StringLit)
	if !ok || len(lit.Parts) != 1 {
		return ""
	}
	return strings.TrimPrefix(lit.Parts[0].Text, rootMark)
}

// loadsServed is the case at path, its loads served from files and replayed by a loadMemoHost.
func loadsServed(t *testing.T, path string, files func(a func(string) []byte) func(string) []byte) *memoed {
	t.Helper()
	m, a := archived(t, path)
	served := files(func(name string) []byte { return archiveFile(a, name) })
	m.load = servedJSON(t, m.p, served)
	m.as = func(h *host) eval.Host { return &loadMemoHost{host: h, files: served} }
	return m
}

// asArchived serves every file as the archive holds it.
func asArchived(a func(string) []byte) func(string) []byte { return a }

// overridden serves one file with the content a test sets, the others as the archive holds them.
type overridden struct {
	name    string
	mu      sync.Mutex
	content string
}

func (o *overridden) serve(a func(string) []byte) func(string) []byte {
	o.content = string(a(o.name))
	return func(name string) []byte {
		if name == o.name {
			return []byte(o.get())
		}
		return a(name)
	}
}

func (o *overridden) get() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.content
}

func (o *overridden) set(content string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.content = content
}

// IMPLEMENTATION-PLAN §7.6, WIRE.md §6, EVALUATION.md §13 (EVL-07): each load recorded, then replayed as cold.
func TestMemoLoadReplays(t *testing.T) {
	for _, path := range loadCases {
		m := loadsServed(t, path, asArchived)
		first, again := twice(t, path, m)
		if first.loads.stored == 0 || again.loads.hits != first.loads.stored || again.loads.stored != 0 {
			t.Errorf("%s: recorded kept %d loads; replayed %d, kept %d", path, first.loads.stored, again.loads.hits, again.loads.stored)
		}
	}
}

// IMPLEMENTATION-PLAN §7.6, WIRE.md §6: a load whose file changed runs again, as the entries reading it.
func TestMemoLoadFileChanges(t *testing.T) {
	rows := &overridden{name: rowsFile}
	m := loadsServed(t, loadsCase, rows.serve)
	orig := rows.get()
	edited := strings.Replace(orig, `"base": 1`, `"base": 2`, 1)
	at := memoAt{memo: eval.NewMemo(), epoch: 1}
	for i, run := range []struct {
		rows       string
		hit, store int
	}{{orig, 0, 2}, {orig, 2, 0}, {edited, 1, 1}, {edited, 2, 0}, {`[{"base": "x"}]`, 1, 0}, {orig, 1, 1}} {
		rows.set(run.rows)
		cold := m.evaluate(t, nil, 0)
		got := m.evaluate(t, at.memo, at.epoch)
		same(t, fmt.Sprint("run ", i), got, cold)
		if got.loads.hits != run.hit || got.loads.stored != run.store {
			t.Errorf("run %d: replayed %d and kept %d loads, want %d and %d", i, got.loads.hits, got.loads.stored, run.hit, run.store)
		}
	}
}

// EVALUATION.md §12.2 (EVL-03): at every budget, a replayed load stops where a cold one does.
func TestMemoLoadBudgetCuts(t *testing.T) {
	m := loadsServed(t, loadsCase, asArchived)
	memo := eval.NewMemo()
	full := m.evaluate(t, memo, 1)
	for budget := int64(1); budget <= full.steps; budget++ {
		m.opt.Budget = budget
		cold := m.evaluate(t, nil, 0)
		same(t, fmt.Sprint("budget ", budget), m.evaluate(t, memo, 1), cold)
	}
}

// EVALUATION.md §7.2 (EVL-05): a load reading a value now poisoned runs again, failing as cold.
func TestMemoLoadPoisonedRead(t *testing.T) {
	evs := &overridden{name: evsFile}
	m := loadsServed(t, loadsCase, evs.serve)
	at := memoAt{memo: eval.NewMemo(), epoch: 1}
	same(t, "decoded", m.evaluate(t, at.memo, at.epoch), m.evaluate(t, nil, 0))
	evs.set(`{"yes": {"on": "x"}, "no": {"on": false}}`)
	cold := m.evaluate(t, nil, 0)
	got := m.evaluate(t, at.memo, at.epoch)
	same(t, "poisoned", got, cold)
	if got.loads.hits != 0 || !strings.Contains(got.dump, "a.rows poisoned") {
		t.Errorf("poisoned: replayed %d loads:\n%s", got.loads.hits, got.dump)
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: two epochs used in turn each replay their own loads and entries.
func TestMemoLoadEpochsInTurn(t *testing.T) {
	m := loadsServed(t, loadsCase, asArchived)
	memo := eval.NewMemo()
	cold := m.evaluate(t, nil, 0)
	for i, epoch := range []uint64{1, 2, 1, 2, 1} {
		got := m.evaluate(t, memo, epoch)
		same(t, fmt.Sprint("run ", i, " epoch ", epoch), got, cold)
		if replayed := i >= 2; (got.loads.hits > 0) != replayed || (got.hits > 0) != replayed {
			t.Errorf("run %d, epoch %d: replayed %d loads and %d entries", i, epoch, got.loads.hits, got.hits)
		}
	}
}

// IMPLEMENTATION-PLAN §7.6, DOCTRINE §5: evaluators replaying one memo's loads concurrently each give the cold outcome.
func TestMemoLoadConcurrent(t *testing.T) {
	m := loadsServed(t, loadsCase, asArchived)
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
			same(t, fmt.Sprint("wave ", wave, " evaluator ", i), out, cold)
		}
	}
}
