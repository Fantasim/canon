package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// CheckTrace is one run of a check on an instance, kept so that a run of that check on a copy
// of the instance (one EntryToken) is replayed: the steps it charged, the values it read with
// their fingerprints, its findings and its outcome.
type CheckTrace struct {
	charge charge
	steps  int64
	reads  []memoRead
	need   int
	files  []*syntax.File
	found  []memoFinding
	out    CheckRun
}

// Outcome is how the traced run went.
func (t *CheckTrace) Outcome() CheckRun {
	return t.out
}

// RunTraced is Run, with the run's trace when a replay reproduces everything it did: it made no
// block report, marked no value and set no identity, read no marked or poisoned value, was not
// tainted, loaded nothing and did not run out of steps. Nil: no trace.
func (e *Evaluator) RunTraced(ctx context.Context, c *syntax.CheckDecl, self value.Value) (CheckRun, *CheckTrace) {
	r := e.checkRun(ctx, c, self)
	if r == nil {
		return CheckRun{Aborted: true}, nil
	}
	u := e.memo
	if u == nil || u.trace != nil {
		return r.check(c), nil
	}
	start, marks := e.spent[r.charge], e.marksGen()
	tr := &entryTrace{run: r, lap: start, depth0: e.depth, implicit0: e.implicit, bugs: len(e.bugs), retagged: e.gens.retagged}
	u.trace = tr
	out := r.check(c)
	u.trace = nil
	if tr.void || r.tainted || e.exhausted || len(e.bugs) != tr.bugs || len(out.Reports) > 0 ||
		e.marksGen() != marks || e.gens.retagged != tr.retagged || !tr.unchanged(e) {
		return out, nil
	}
	return out, &CheckTrace{charge: r.charge, steps: e.spent[r.charge] - start, reads: tr.reads, need: tr.need, files: tr.files, found: tr.found, out: out}
}

// ReplayChecks replays traces in order, each value they read forced and unchanged, their code
// the program's and their steps within the budget: each one's steps charged to its check and its
// findings reported, as its run did. False: nothing was done; the caller runs the checks.
func (e *Evaluator) ReplayChecks(ctx context.Context, traces []*CheckTrace) bool {
	u := e.memo
	if u == nil || u.trace != nil || e.exhausted || ctx.Err() != nil {
		return false
	}
	var total int64
	for _, tr := range traces {
		if !e.replayable(tr) {
			return false
		}
		total += tr.steps
	}
	if e.steps+total >= e.budget {
		return false // a run reports E4401 at its own expression
	}
	for _, tr := range traces {
		e.chargeKept(tr.charge, tr.steps)
		for _, f := range tr.found {
			e.report(f.pkg, f.b)
		}
	}
	return true
}

// replayable reports that tr's calls stay under the depth limit, its code is the program's, and
// each value it read is forced, unmarked and the one it read.
func (e *Evaluator) replayable(tr *CheckTrace) bool {
	if !e.canReplay(&memoEntry{need: tr.need, files: tr.files}) {
		return false
	}
	for i := range tr.reads {
		rd := &tr.reads[i]
		st := e.rootState(rd.root)
		if st == nil || st.status != done {
			return false
		}
		in := e.memo.info(e, st.v)
		if !in.ok || in.fp != rd.fp || !in.cleanIn(e) {
			return false
		}
	}
	return true
}

// chargeKept charges n steps to c as the runs of a replayed check spent them (EVALUATION.md §12.2).
func (e *Evaluator) chargeKept(c charge, n int64) {
	if n == 0 {
		return
	}
	if e.spent[c] == 0 {
		e.order = append(e.order, c)
	}
	e.steps += n
	e.spent[c] += n
}
