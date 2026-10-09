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

// loadKey is what a load's evaluation depends on besides the files it reads and the values it forces:
// its expression (a changed file brings new nodes), its type, the let collection it is the whole of,
// the field scope it decodes in, the active layers, and whether the root was already tainted.
type loadKey struct {
	expr    *syntax.LoadExpr
	t       types.Type
	coll    *types.Collection
	scope   scopeKey
	layers  string
	tainted bool
}

// scopeKey is what a field scope changes in a decoding, zero for none (WIRE.md §4.1).
type scopeKey struct {
	unit types.Unit
	enc  types.Enc
	none string
}

// scopeKeyOf is f's scope key.
func scopeKeyOf(f *types.Field) scopeKey {
	if f == nil {
		return scopeKey{}
	}
	return scopeKey{unit: f.Unit, enc: f.Enc, none: string(f.NoneWire)}
}

// loadEntry is one load's evaluation: what the host recorded, the file holding the load, and the
// steps, reads, code, findings and marked value as an entry's, the value at its graph's root.
type loadEntry struct {
	in   LoadInputs
	file *syntax.File
	en   memoEntry
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
	return lm, loadKey{expr: e, t: t, coll: site.coll, scope: scopeKeyOf(site.field), layers: u.layers, tainted: r.tainted}, true
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
	start, failed := e.spentOn(r.charge), r.failed
	e.loading = site
	infos, out, said := r.replayReads(&le.en)
	e.loading = nil
	var tab []value.Value
	if out == replayOn {
		tab, same = le.en.kept.seed(infos)
	}
	if out != replayOn || !same { // a value read now poisoned fails the load, which loading reports
		r.failed = failed
		r.rollback(start)
		return nil, false
	}
	e.memo.loaded.hits++
	done()
	r.reportKept(le.en.found, said)
	return e.thaw(&le.en.kept, tab), true
}

// recordLoad runs the load through the host, recorded as an entry is.
func (r *run) recordLoad(lm LoadMemo, k loadKey, site *loadSite) value.Value {
	u := r.ev.memo
	tr := r.startTrace(memoKey{tainted: k.tainted})
	ps := u.partsOf(r, tr, k)
	site.parts = ps
	r.ev.loading = site
	v, ok, in := lm.LoadRecorded(r.ctx, k.expr, k.t)
	r.ev.loading, site.parts = nil, nil
	v = r.loaded(v, ok, k.t)
	u.trace = nil
	u.storeParts(ps, r.fr.file)
	if ps.used && ps.whole { // its elements are kept apart: the next load serves them again
		return v
	}
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
	if v == nil || in == nil || tr.void || e.stopped() || len(e.bugs) != tr.bugs || tr.run.tainted != tr.key.tainted || !tr.unchanged(e) {
		return nil, false
	}
	kept, ok := e.freezeValue(v, tr.infos)
	if !ok {
		return nil, false
	}
	en := memoEntry{reads: tr.reads, tail: tr.steps(e), need: tr.need, files: tr.files, found: tr.found, kept: kept}
	en.size = memoNodeBytes * (1 + len(en.reads) + len(en.found) + kept.size)
	return &loadEntry{in: in, en: en}, true
}

// freezeValue is v kept apart as an entry's record is, v's copy (v itself when shared as it is)
// at its root; false when v holds a function value or is a value read.
func (e *Evaluator) freezeValue(v value.Value, reads []*readInfo) (memoGraph, bool) {
	f := e.newFreezer(v, reads)
	if !f.walk() {
		return memoGraph{}, false
	}
	if _, read := f.foreign[v]; read {
		return memoGraph{}, false
	}
	return f.kept(v), true
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

// forget empties g of its entries, loads and load.dir elements.
func (g *memoGen) forget() {
	g.entries, g.loads, g.parts, g.bytes = map[memoKey]*memoEntry{}, map[loadKey]*loadEntry{}, map[loadKey]*partSet{}, 0
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
