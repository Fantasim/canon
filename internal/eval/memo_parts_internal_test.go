package eval

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	partsBudget = 100 // the test evaluator's steps
	partsSteps  = 3   // what decoding the test element charges
	partsSrc    = "a.json"
)

// partsUnder is an evaluator using a new memo in epoch 1, recording a load in a run of its own.
func partsUnder(t *testing.T) (*Evaluator, *run, *Parts) {
	t.Helper()
	e := newEvaluator(check.Bags{}, Options{Budget: partsBudget})
	m := NewMemo()
	e.memo = &memoUse{m: m, gen: m.begin(1, liveness{}), reads: map[value.Value]*readInfo{}}
	r := e.newRun(context.Background(), charge{pkg: partsSrc, name: partsSrc}, nil)
	return e, r, e.memo.partsOf(r, r.startTrace(memoKey{}), loadKey{})
}

// partsRecord is a decoded element.
func partsRecord() *value.Record {
	return &value.Record{Fields: []value.Value{&value.Int{V: 1}}, Set: []bool{true}}
}

// partDoing is one thing an element's decoding may do besides making it and charging its steps.
var partDoing = []struct {
	name string
	do   func(e *Evaluator, r *run, tr *entryTrace)
}{
	{"a value read", func(_ *Evaluator, _ *run, tr *entryTrace) { tr.reads = append(tr.reads, memoRead{}) }},
	{"steps charged elsewhere", func(e *Evaluator, _ *run, _ *entryTrace) {
		e.newRun(context.Background(), charge{name: partsSrc}, nil).spend(1, noSpan)
	}},
	{"a finding", func(_ *Evaluator, _ *run, tr *entryTrace) { tr.found = append(tr.found, memoFinding{}) }},
	{"a value marked", func(e *Evaluator, _ *run, _ *entryTrace) { e.invalid[&value.Int{}] = true }},
	{"a written mark", func(e *Evaluator, _ *run, _ *entryTrace) { e.gens.written++ }},
	{"a copy's origin", func(e *Evaluator, _ *run, _ *entryTrace) { e.origin[partsRecord()] = partsRecord() }},
	{"arguments bound", func(e *Evaluator, _ *run, _ *entryTrace) { e.bound[partsRecord()] = nil }},
	{"an internal error", func(e *Evaluator, _ *run, _ *entryTrace) { e.bugs = append(e.bugs, context.Canceled) }},
	{"a stable amendment", func(e *Evaluator, _ *run, _ *entryTrace) { e.stable = append(e.stable, StableAmendment{}) }},
	{"an identity set in place", func(e *Evaluator, _ *run, _ *entryTrace) { e.gens.retagged++ }},
	{"the recording voided", func(_ *Evaluator, _ *run, tr *entryTrace) { tr.void = true }},
	{"an abort", func(_ *Evaluator, _ *run, tr *entryTrace) { tr.aborted = true }},
	{"the run failed", func(_ *Evaluator, r *run, _ *entryTrace) { r.failed = true }},
	{"the run tainted", func(_ *Evaluator, r *run, _ *entryTrace) { r.tainted = true }},
	{"the budget spent", func(e *Evaluator, _ *run, _ *entryTrace) { e.exhausted = true }},
}

// IMPLEMENTATION-PLAN §7.6 NFR-02: an element is kept only when decoding it did nothing else.
func TestPartsKeptOnlyPure(t *testing.T) {
	keep := func(do func(*Evaluator, *run, *entryTrace), pure bool) (*Parts, *memoEntry) {
		e, r, p := partsUnder(t)
		done := p.Start()
		r.spend(partsSteps, noSpan)
		do(e, r, p.tr)
		done(partsSrc, partsRecord(), false, pure)
		return p, p.now[partsSrc]
	}
	nothing := func(*Evaluator, *run, *entryTrace) {}
	if p, en := keep(nothing, true); en == nil || en.tail != partsSteps || !p.whole {
		t.Fatalf("a pure element: kept %v, want kept with its %d steps", en, partsSteps)
	}
	if p, en := keep(nothing, false); en != nil || p.whole {
		t.Error("impure on the decoder's side: kept")
	}
	for _, d := range partDoing {
		if p, en := keep(d.do, true); en != nil || p.whole {
			t.Errorf("%s: kept", d.name)
		}
	}
}

// EVALUATION.md §12.2, IMPLEMENTATION-PLAN §7.6: served as a copy, charged, unless over budget or failed.
func TestPartsServed(t *testing.T) {
	e, r, p := partsUnder(t)
	done := p.Start()
	r.spend(partsSteps, noSpan)
	rec := partsRecord()
	done(partsSrc, rec, true, true)
	serve := func(steps int64, failed bool) (*value.Record, bool, bool) {
		e.steps, e.spent[r.charge], r.failed = steps, steps, failed
		p.kept, p.now = map[any]*memoEntry{partsSrc: p.now[partsSrc]}, map[any]*memoEntry{}
		return p.Kept(partsSrc)
	}
	if got, retired, ok := serve(0, false); !ok || got == rec || !retired || got.Fields[0] != rec.Fields[0] || e.steps != partsSteps {
		t.Errorf("served %v %t %t, %d steps: want a copy, retired, its steps charged", got, retired, ok, e.steps)
	}
	if _, _, ok := serve(partsBudget-partsSteps, false); ok || e.steps != partsBudget-partsSteps {
		t.Error("steps reaching the budget: served, or charged")
	}
	if _, _, ok := serve(0, true); ok || e.steps != 0 {
		t.Error("a failed run: served, or charged")
	}
}
