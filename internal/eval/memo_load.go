package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// LoadMemo is a Host whose loads the memo replays (IMPLEMENTATION-PLAN §7.6).
type LoadMemo interface {
	// LoadRecorded is Load, and what a replay needs of it: nil when none could reproduce it.
	LoadRecorded(ctx context.Context, e *syntax.LoadExpr, expected types.Type) (value.Value, bool, LoadInputs)
	// LoadReplay reads in again as the load e read it, false at the first answer that differs;
	// done then reports what the load reported itself.
	LoadReplay(ctx context.Context, e *syntax.LoadExpr, in LoadInputs) (done func(), ok bool)
}

// LoadInputs is what a LoadMemo recorded of one load, which the memo keeps for it.
type LoadInputs any

// loadKey is what a load's evaluation depends on besides the files it reads and the values it
// forces: its expression (a changed file brings new nodes), its type, the let collection it is
// the whole of, the active layers, and whether the root was already tainted.
type loadKey struct {
	expr    *syntax.LoadExpr
	t       types.Type
	coll    *types.Collection
	layers  string
	tainted bool
}

// loadEntry is one load's evaluation: what the host recorded, the file holding the load, the
// steps, reads, code, findings and marked value as an entry's, and the value's own copy in it.
type loadEntry struct {
	in   LoadInputs
	file *syntax.File
	en   memoEntry
	root value.Value
}

// load forces the load e as t: replayed while what it reads is unchanged, else run and recorded.
func (r *run) load(e *syntax.LoadExpr, t types.Type, site *loadSite) value.Value {
	lm, k, memo := r.loadMemo(e, t, site)
	if !memo {
		outer := r.ev.loading
		r.ev.loading = site
		v, ok := r.ev.host.Load(r.ctx, e, t)
		r.ev.loading = outer
		return r.loaded(v, ok, t)
	}
	if v, hit := r.replayLoad(lm, k, site); hit {
		return v
	}
	return r.recordLoad(lm, k, site)
}

// loaded is a load's value, its dependent parts marked; nil poisons silently (EVALUATION.md §7.1).
func (r *run) loaded(v value.Value, ok bool, t types.Type) value.Value {
	if !ok || r.failed {
		r.stop()
		return nil
	}
	r.ev.markLoaded(v, t)
	return v
}

// loadMemo is the host's memo side and e's key, when the evaluator uses a memo and the load is a
// plain one: no entry recorded and no load decoded around it, a root's own frame, its type
// naming no record, argument or binder around it (DECISIONS 173).
func (r *run) loadMemo(e *syntax.LoadExpr, t types.Type, site *loadSite) (LoadMemo, loadKey, bool) {
	u := r.ev.memo
	lm, ok := r.ev.host.(LoadMemo)
	if !ok || u == nil || u.trace != nil || !r.ev.plain() || !r.plainLoad(site.cx) {
		return nil, loadKey{}, false
	}
	return lm, loadKey{expr: e, t: t, coll: site.coll, layers: u.layers, tainted: r.tainted}, true
}

// plainLoad reports a run of a top-level value in its root frame, outside stage B, tests, views
// and amendments, whose load's type names nothing around it.
func (r *run) plainLoad(cx *depCtx) bool {
	return r.sink == nil && r.emitted == nil && r.test == nil && r.magic == nil && r.cmp == nil &&
		r.reports == nil && !r.free && r.mv == nil && r.fr.caller == nil && r.fr.fn == "" &&
		cx.rec == nil && cx.params == nil && cx.binders == nil
}

// replayLoad replays the load kept under k: the host reads its files again, then, as for an
// entry, its steps around a new forcing of each value it read, its findings, marks and value.
// Up to a difference it did what loading does; there its steps are taken back.
func (r *run) replayLoad(lm LoadMemo, k loadKey, site *loadSite) (value.Value, bool) {
	e := r.ev
	le := e.memo.lookupLoad(k)
	if le == nil || !e.canReplay(&le.en) {
		return nil, false
	}
	done, same := lm.LoadReplay(r.ctx, k.expr, le.in)
	if !same {
		return nil, false
	}
	start, failed := e.spent[r.charge], r.failed
	e.loading = site
	infos, out, said := r.replayReads(&le.en)
	e.loading = nil
	var copies map[value.Value]value.Value
	if out == replayOn {
		copies, same = le.en.kept.seed(infos)
	}
	if out != replayOn || !same { // a value read now poisoned fails the load, which loading reports
		r.failed = failed
		r.rollback(start)
		return nil, false
	}
	e.memo.loaded.hits++
	done()
	r.reportKept(le.en.found, said)
	e.thaw(le.en.kept, copies)
	return get(copies, le.root), true
}

// recordLoad runs the load through the host, recorded as an entry is.
func (r *run) recordLoad(lm LoadMemo, k loadKey, site *loadSite) value.Value {
	tr := r.startTrace(memoKey{tainted: k.tainted})
	r.ev.loading = site
	v, ok, in := lm.LoadRecorded(r.ctx, k.expr, k.t)
	r.ev.loading = nil
	v = r.loaded(v, ok, k.t)
	r.ev.memo.trace = nil
	u := r.ev.memo
	le, kept := tr.loadEntry(r.ev, v, in)
	if !kept {
		u.loaded.unkept++
		return v
	}
	le.file = r.fr.file
	u.storeLoad(k, le)
	return v
}

// loadEntry is what tr recorded of a load giving v, false when a replay could not reproduce it:
// a failure, inputs the host could not record, or what an entry's recording refuses.
func (tr *entryTrace) loadEntry(e *Evaluator, v value.Value, in LoadInputs) (*loadEntry, bool) {
	if v == nil || in == nil || tr.void || e.exhausted || len(e.bugs) != tr.bugs || tr.run.tainted != tr.key.tainted || !tr.unchanged(e) {
		return nil, false
	}
	kept, root, ok := e.freezeValue(v, tr.infos)
	if !ok {
		return nil, false
	}
	en := memoEntry{reads: tr.reads, tail: tr.steps(e), need: tr.need, files: tr.files, found: tr.found, kept: kept}
	en.size = memoNodeBytes * (1 + len(en.reads) + len(en.found) + kept.size)
	return &loadEntry{in: in, en: en, root: root}, true
}

// freezeValue is v kept apart as an entry's record is, with v's own copy in it (v itself when
// shared as it is); false when v holds a function value or is a value read.
func (e *Evaluator) freezeValue(v value.Value, reads []*readInfo) (memoGraph, value.Value, bool) {
	f := &freezer{e: e, reads: reads, seen: map[value.Value]bool{}, stack: []value.Value{v}, foreign: map[value.Value]foreignAt{}, ok: true}
	for len(f.stack) > 0 && f.ok {
		n := f.stack[len(f.stack)-1]
		f.stack = f.stack[:len(f.stack)-1]
		f.visit(n)
	}
	if _, read := f.foreign[v]; !f.ok || read {
		return memoGraph{}, nil, false
	}
	g := f.kept(nil)
	if shell(v) == nil {
		return g, v, true
	}
	return g, g.nodes[0], true
}

// lookupLoad is the load kept under k, nil for none.
func (u *memoUse) lookupLoad(k loadKey) *loadEntry {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	return u.gen.loads[k]
}

// storeLoad keeps le under k, within the memo's bounds as store keeps an entry.
func (u *memoUse) storeLoad(k loadKey, le *loadEntry) {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	g := u.gen
	if old := g.loads[k]; old != nil {
		g.bytes -= old.en.size
		delete(g.loads, k)
	}
	if !u.m.admit(g, le.en.size, len(g.loads) >= memoCap) {
		u.loaded.unkept++
		return
	}
	g.loads[k] = le
	g.bytes += le.en.size
	u.loaded.stored++
}

// dropLoads deletes the loads of files no longer alive.
func (g *memoGen) dropLoads(alive func(*syntax.File) bool) {
	for k, le := range g.loads { //canon:unordered deleting the dead ones, in any order
		if !alive(le.file) {
			g.bytes -= le.en.size
			delete(g.loads, k)
		}
	}
}

// forget empties g of its entries and loads.
func (g *memoGen) forget() {
	g.entries, g.loads, g.bytes = map[memoKey]*memoEntry{}, map[loadKey]*loadEntry{}, 0
}

// admit makes room in g for n more bytes: g emptied when full or past memoBytes, then the other
// stores, least recent first, as far as memoAllBytes needs; false for more than memoBytes, which
// is not kept (log-2026-09-29 M4 P3-r).
func (m *Memo) admit(g *memoGen, n int, full bool) bool {
	if n > memoBytes {
		return false
	}
	if full || g.bytes+n > memoBytes {
		g.forget()
	}
	total := g.bytes + n
	for _, x := range m.gens {
		if x != g {
			total += x.bytes
		}
	}
	for i := len(m.gens) - 1; i >= 0 && total > memoAllBytes; i-- {
		if x := m.gens[i]; x != g {
			total -= x.bytes
			x.forget()
		}
	}
	return true
}
