package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// entryTrace records an entry being evaluated: the steps charged around each value it read, the
// frames its calls reached, the code it ran, its findings; void, anything a replay could not
// reproduce (a cycle, the depth limit, a load, a poisoned read, another run's finding).
type entryTrace struct {
	run       *run
	key       memoKey
	lap       int64
	depth0    int
	implicit0 int
	need      int
	bugs      int
	retagged  int
	reads     []memoRead
	infos     []*readInfo
	files     []*syntax.File
	found     []memoFinding
	aborted   bool
	void      bool
}

// memoKey is the key of the entry d, when the evaluator uses a memo and the run is a plain
// evaluation of a let's entries: no test, vector, fold, capture, load or phase-8 read.
func (r *run) memoKey(d *syntax.EntryDecl, t types.Type, coll *types.Collection, saved *frame) (memoKey, bool) {
	u := r.ev.memo
	if u == nil || u.trace != nil || saved.caller != nil || !r.ev.plain() || !r.plain() {
		return memoKey{}, false
	}
	return memoKey{decl: d, t: t, coll: coll, layers: u.layers, tainted: r.tainted}, true
}

// plain reports an evaluator reporting into its bags, outside tests, vectors, folds and loads.
func (e *Evaluator) plain() bool {
	return e.info != nil && e.parent == nil && e.vec == nil && e.aside == nil && e.recording == nil &&
		e.testStops == nil && e.causes == nil && e.loading == nil && !e.constant && !e.late.free
}

// plain reports a run of a top-level value, not a capture, stage B, test or view read.
func (r *run) plain() bool {
	return r.sink == nil && r.emitted == nil && r.test == nil && r.magic == nil && r.cmp == nil &&
		r.reports == nil && !r.free && r.dep == nil && r.coll == nil && r.mv == nil
}

// startTrace starts recording the entry of key k, after its first step.
func (r *run) startTrace(k memoKey) *entryTrace {
	e := r.ev
	tr := &entryTrace{run: r, key: k, lap: e.spent[r.charge], depth0: e.depth, implicit0: e.implicit, bugs: len(e.bugs), retagged: e.gens.retagged}
	e.memo.trace = tr
	return tr
}

// finishTrace ends the recording and keeps the entry, rec or its failure, when a replay can
// reproduce it.
func (r *run) finishTrace(tr *entryTrace, rec *value.Record) {
	u := r.ev.memo
	u.trace = nil
	en, ok := tr.entry(r.ev, rec)
	if !ok {
		u.stats.unkept++
		return
	}
	u.store(tr.key, en)
	u.noteToken(rec, en)
}

// entry is what tr recorded, false when a replay could not reproduce it: a stop without a
// finding, the budget spent, an internal error, a taint, or a value read then marked.
func (tr *entryTrace) entry(e *Evaluator, rec *value.Record) (*memoEntry, bool) {
	r := tr.run
	if tr.void || e.exhausted || len(e.bugs) != tr.bugs || r.tainted != tr.key.tainted {
		return nil, false
	}
	if (rec == nil) != r.failed || rec == nil && !tr.aborted {
		return nil, false
	}
	if !tr.unchanged(e) {
		return nil, false
	}
	en := &memoEntry{reads: tr.reads, tail: tr.steps(e), need: tr.need, files: tr.files, found: tr.found}
	if rec == nil {
		en.size = memoNodeBytes * (1 + len(en.reads) + len(en.found))
		return en, true
	}
	kept, ok := e.freeze(rec, tr.infos)
	en.kept = kept
	en.size = memoNodeBytes * (1 + len(en.reads) + len(en.found) + kept.size)
	return en, ok
}

// unchanged reports that no value read was marked since, nor retagged: once an identity was set
// in place during the entry (colls.go), each value read is fingerprinted again, since the entry
// may have retagged a node of it, which a replay would not do.
func (tr *entryTrace) unchanged(e *Evaluator) bool {
	for i, in := range tr.infos {
		if !in.cleanIn(e) {
			return false
		}
		if e.gens.retagged != tr.retagged && e.fingerprintOf(in.nodes[0]).fp != tr.reads[i].fp {
			return false
		}
	}
	return true
}

// steps is the steps charged to the entry since the last read.
func (tr *entryTrace) steps(e *Evaluator) int64 {
	n := e.spent[tr.run.charge] - tr.lap
	tr.lap += n
	return n
}

// trace is the recording of the entry r evaluates, nil for none.
func (r *run) trace() *entryTrace {
	if u := r.ev.memo; u != nil && u.trace != nil && u.trace.run == r {
		return u.trace
	}
	return nil
}

// forceRead forces a top-level value r reads; while an entry is recorded, the value is forced
// with the recording set aside, and noted with the steps charged before it.
func (r *run) forceRead(st *rootState, at syntax.Node) (value.Value, bool) {
	tr := r.trace()
	if tr == nil {
		return r.ev.force(r.ctx, st, r, at)
	}
	e := r.ev
	rd := memoRead{steps: tr.steps(e), root: st.root, depth: e.depth - tr.depth0, implicit: e.implicit - tr.implicit0}
	if st.status == forcing {
		tr.void = true // E4301 names the values being forced (EVALUATION.md §3.2)
	}
	e.memo.trace = nil
	v, ok := e.force(r.ctx, st, r, at)
	e.memo.trace = tr
	tr.read(e, rd, v, ok)
	return v, ok
}

// read notes a value read, which a replay compares by fingerprint; a poisoned or marked one,
// or a function value, voids the recording.
func (tr *entryTrace) read(e *Evaluator, rd memoRead, v value.Value, ok bool) {
	if !ok {
		tr.void = true
		return
	}
	in := e.memo.info(e, v)
	if !in.ok || !in.cleanIn(e) {
		tr.void = true
		return
	}
	rd.fp = in.fp
	tr.reads, tr.infos = append(tr.reads, rd), append(tr.infos, in)
}

// noteFinding keeps a finding the recorded entry reports, detached from the values it names;
// another run's voids the recording.
func (r *run) noteFinding(b *diag.Builder) {
	u := r.ev.memo
	if u == nil || u.trace == nil {
		return
	}
	if u.trace.run != r {
		u.trace.void = true
		return
	}
	u.trace.found = append(u.trace.found, memoFinding{pkg: r.fr.pkg, b: b.Detached(), read: len(u.trace.reads)})
}

// noteAbort notes that the recorded entry stops on a hard error (EVALUATION.md §7.1).
func (r *run) noteAbort() {
	if tr := r.trace(); tr != nil {
		tr.aborted = true
	}
}

// noteDepth notes the frames a call of the recorded entry reaches above the entry's own (EVALUATION.md §3.3).
func (r *run) noteDepth() {
	if tr := r.trace(); tr != nil {
		tr.need = max(tr.need, r.ev.depth-tr.depth0+1)
	}
}

// noteCode notes a file whose code the recorded entry runs: a replay needs it unchanged.
func (r *run) noteCode(file *syntax.File) {
	if tr := r.trace(); tr != nil {
		tr.code(file)
	}
}

// noteType notes the file declaring a record or case type the recorded entry builds: its
// defaults, where predicates and field declarations.
func (r *run) noteType(t types.Type) {
	if tr := r.trace(); tr != nil {
		tr.code(r.ev.declFile(t))
	}
}

func (tr *entryTrace) code(file *syntax.File) {
	if file != nil && !slices.Contains(tr.files, file) {
		tr.files = append(tr.files, file)
	}
}

// voidTrace voids the recording of the entry r evaluates, if any.
func (r *run) voidTrace() {
	if u := r.ev.memo; u != nil && u.trace != nil {
		u.trace.void = true
	}
}
