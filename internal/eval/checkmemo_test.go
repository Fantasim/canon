package eval_test

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

const (
	checksCase  = "testdata/checkmemo/checks.txtar"
	checksCfg   = "resource/cfg.json"
	checksLower = `{"lim": 2}`
	checksPkg   = "a"
	checksRows  = "rows"
	checksEpoch = 1
)

// rowRuns is an evaluator of the checks case after stage B, its bags, and each (check, row) to run.
type rowRuns struct {
	b    *build
	runs []rowRun
}

// rowRun is one check to run on one row.
type rowRun struct {
	c    *syntax.CheckDecl
	rec  *value.Record
	path string // the row's canonical path, that of a hard error its check raises (API.md F1)
}

// rowsAfterB evaluates m's values and verifies them, with memo in the checks epoch when set.
func rowsAfterB(t *testing.T, m *memoed, memo *eval.Memo, opt eval.Options) *rowRuns {
	t.Helper()
	ctx := context.Background()
	b := &build{prog: m.p, bags: check.Bags{}, checked: m.prog, values: map[eval.Root]value.Value{}}
	for _, name := range m.bags {
		b.bags[name] = diag.NewBag(m.p.fs, name)
	}
	h := &host{load: m.load}
	b.ev = eval.New(b.checked, h, b.bags, opt)
	if memo != nil {
		b.ev.UseMemo(memo, checksEpoch)
	}
	h.ev, h.verifier = b.ev, verify.New(b.ev, b.checked, b.bags, nil)
	for _, pkg := range b.checked.Packages {
		for _, obj := range pkg.Decls {
			if obj.Kind() == check.ObjConst || obj.Kind() == check.ObjLet {
				b.ev.Force(ctx, eval.Root{Pkg: pkg.Path, Name: obj.Name()}) // stage A, as a build forces them
			}
		}
	}
	b.ev.BeginVerification(ctx)
	v, ok := b.ev.Force(ctx, eval.Root{Pkg: checksPkg, Name: checksRows})
	rows, isTable := v.(*value.Table)
	if !ok || !isTable {
		t.Fatalf("rows: %v", v)
	}
	out := &rowRuns{b: b}
	for _, rec := range rows.Entries {
		for _, c := range rec.T.Base().(*types.RecordType).Checks {
			out.runs = append(out.runs, rowRun{c: c, rec: rec, path: checksRows + "." + rec.Ident.Key.S})
		}
	}
	return out
}

// spent is what f charged in r's evaluator.
func (r *rowRuns) spent(f func()) int64 {
	before := r.b.ev.StepsSpent()
	f()
	return r.b.ev.StepsSpent() - before
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, EVALUATION.md §8.4, §12.2: check runs traced, then replayed.
func TestCheckRunsReplay(t *testing.T) {
	m, _ := archived(t, checksCase)
	cold := rowsAfterB(t, m, nil, m.opt)
	var want []eval.CheckRun
	coldSteps := cold.spent(func() {
		for _, x := range cold.runs {
			want = append(want, cold.b.ev.Run(context.Background(), x.c, x.rec, x.path))
		}
	})
	if f := cold.b.findings(t); !strings.Contains(f, "rows.c: ") { // API.md F1: the hard error of c's message
		t.Fatalf("no finding at rows.c:\n%s", f)
	}
	memo := eval.NewMemo()
	traced := rowsAfterB(t, m, memo, m.opt)
	var traces []*eval.CheckTrace
	var got []eval.CheckRun
	tracedSteps := traced.spent(func() {
		for _, x := range traced.runs {
			out, tr := traced.b.ev.RunTraced(context.Background(), x.c, x.rec, x.path)
			got, traces = append(got, out), append(traces, tr)
		}
	})
	if !reflect.DeepEqual(got, want) || tracedSteps != coldSteps || traced.b.findings(t) != cold.b.findings(t) {
		t.Fatalf("traced runs %v charged %d, cold %v charged %d", got, tracedSteps, want, coldSteps)
	}
	for i, tr := range traces {
		if tr == nil || !reflect.DeepEqual(tr.Outcome(), want[i]) {
			t.Fatalf("run %d: trace %v", i, tr)
		}
	}
	replayed(t, m, memo, traces, ranAs{steps: coldSteps, findings: cold.b.findings(t)})
	short := m.opt
	short.Budget = cold.b.ev.StepsSpent()
	if r := rowsAfterB(t, m, memo, short); r.spent(func() { replayWants(t, r, traces, false) }) != 0 {
		t.Error("a replay past the budget charged steps")
	}
	m.load = servedJSON(t, m.p, func(string) []byte { return []byte(checksLower) })
	if r := rowsAfterB(t, m, memo, m.opt); r.spent(func() { replayWants(t, r, traces, false) }) != 0 {
		t.Errorf("a replay of changed %s charged steps", checksCfg)
	}
}

// replayed replays traces on evaluators at once, each charging steps and reporting findings as the runs did.
func replayed(t *testing.T, m *memoed, memo *eval.Memo, traces []*eval.CheckTrace, want ranAs) {
	t.Helper()
	var wg sync.WaitGroup
	got := make([]*rowRuns, parallelRuns)
	for i := range got {
		got[i] = rowsAfterB(t, m, memo, m.opt)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if n := got[i].spent(func() { replayWants(t, got[i], traces, true) }); n != want.steps {
				t.Errorf("replay %d charged %d steps, the runs %d", i, n, want.steps)
			}
		}()
	}
	wg.Wait()
	for i, r := range got {
		if f := r.b.findings(t); f != want.findings {
			t.Errorf("replay %d reported:\n%s\nthe runs:\n%s", i, f, want.findings)
		}
	}
}

// ranAs is what the runs a replay reproduces charged and reported.
type ranAs struct {
	steps    int64
	findings string
}

// replayWants replays traces on r's evaluator and wants ok.
func replayWants(t *testing.T, r *rowRuns, traces []*eval.CheckTrace, ok bool) {
	t.Helper()
	if got := r.b.ev.ReplayChecks(context.Background(), traces); got != ok {
		t.Errorf("replayed %t, want %t", got, ok)
	}
}
