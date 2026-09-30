package eval

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Parts are the elements of a load.dir being recorded, each file's kept as an entry's evaluation
// is: served again, its steps charged, while its parsed tree is the same and decoding it did
// nothing but make it. A changed file costs its own decoding (NFR-02).
type Parts struct {
	r     *run
	tr    *entryTrace
	key   loadKey
	kept  map[any]*memoEntry // the store's elements of the load when it began, never changed
	now   map[any]*memoEntry // those served or made since
	used  bool               // a load.dir asked for them
	whole bool               // every element was served or made: the load keeps no record of its own
	made  int                // the elements decoded
}

// partHome is where the store keeps an element: its load, the key of its source, and whether a
// table's entry was retired.
type partHome struct {
	load    loadKey
	src     any
	retired bool
}

// partSet is a load's elements in the store, by the key of their source, and the file holding
// the load. Its map is never changed once stored.
type partSet struct {
	elems map[any]*memoEntry
	file  *syntax.File
}

// partState is what decoding an element may change besides its steps: a pure element changes
// none of it.
type partState struct {
	marks, origin, bound, bugs, stable, retagged, reads, found int
	void, aborted, failed, tainted, exhausted                  bool
}

// LoadParts is the kept elements, of type elem, of the load.dir being recorded; nil when the
// load is not recorded, or its elements are not records or hold a dependent type, which
// loading marks as written (stageb.go).
func (e *Evaluator) LoadParts(elem types.Type) *Parts {
	ld, u := e.loading, e.memo
	if ld == nil || ld.parts == nil || u == nil || u.trace != ld.parts.tr || !recordElems(elem) || e.dependent(elem) {
		return nil
	}
	ld.parts.used = true
	return ld.parts
}

// recordElems reports an element type whose values are records: a record's or a variant's.
func recordElems(t types.Type) bool {
	switch t.Base().(type) {
	case *types.RecordType, *types.VariantType:
		return true
	}
	return false
}

// partsOf is the kept elements of the load k that r records under tr.
func (u *memoUse) partsOf(r *run, tr *entryTrace, k loadKey) *Parts {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	p := &Parts{r: r, tr: tr, key: k, now: map[any]*memoEntry{}, whole: true}
	if ps := u.gen.parts[k]; ps != nil {
		p.kept = ps.elems
	}
	return p
}

// Kept is a copy of the element kept for src, its steps charged, and whether a table's entry is
// retired; false, nothing done, when none is kept or decoding would differ: the run failed or
// tainted, calls too deep, code gone, or the steps reaching the budget (E4401 at its expression).
func (p *Parts) Kept(src any) (*value.Record, bool, bool) {
	en := p.kept[src]
	r, e := p.r, p.r.ev
	if en == nil || r.failed || e.exhausted || r.tainted != p.key.tainted || !e.canReplay(en) {
		return nil, false, false
	}
	if r.replaySteps(en.tail) != replayOn {
		return nil, false, false
	}
	for _, f := range en.files {
		p.tr.code(f)
	}
	p.tr.need = max(p.tr.need, en.need)
	rec := e.thaw(en.kept, map[value.Value]value.Value{})
	p.now[src] = en
	e.memo.noteToken(rec, en)
	e.memo.parted.hits++
	return rec, en.part.retired, true
}

// Start begins decoding an element no copy was served for; done is told its source's key, the
// element (nil when decoding failed), whether a table's entry is retired, and whether decoding
// it was pure on the decoder's side, and keeps it when it was pure on the evaluator's too.
func (p *Parts) Start() (done func(src any, rec *value.Record, retired, pure bool)) {
	r, tr := p.r, p.tr
	e := r.ev
	at, steps, spent := e.partState(r, tr), e.steps, e.spent[r.charge]
	files, need := tr.files, tr.need
	tr.files, tr.need = nil, 0
	return func(src any, rec *value.Record, retired, pure bool) {
		own, ownNeed := tr.files, tr.need
		tr.files, tr.need = files, max(need, ownNeed)
		for _, f := range own {
			tr.code(f)
		}
		p.made++
		n := e.spent[r.charge] - spent
		if !pure || rec == nil || e.partState(r, tr) != at || e.steps-steps != n {
			p.whole = false
			return
		}
		kept, ok := e.freeze(rec, nil)
		if !ok {
			p.whole = false
			return
		}
		en := &memoEntry{tail: n, need: ownNeed, files: own, kept: kept, owner: r.charge, part: &partHome{load: p.key, src: src, retired: retired}}
		en.size = memoNodeBytes * (1 + kept.size)
		p.now[src] = en
		e.memo.noteToken(rec, en)
		e.memo.parted.stored++
	}
}

// partState is what an element's decoding must leave as it found it, besides steps.
func (e *Evaluator) partState(r *run, tr *entryTrace) partState {
	return partState{
		marks: e.marksGen(), origin: len(e.origin), bound: len(e.bound), bugs: len(e.bugs), stable: len(e.stable),
		retagged: e.gens.retagged, reads: len(tr.reads), found: len(tr.found),
		void: tr.void, aborted: tr.aborted, failed: r.failed, tainted: r.tainted, exhausted: e.exhausted,
	}
}

// storeParts keeps p's elements as the load's, in place of those it had, when a load.dir asked
// for them. A load whose every element was served or made keeps no record of its own: the next
// one runs again and serves them.
func (u *memoUse) storeParts(p *Parts, file *syntax.File) {
	if !p.used {
		return
	}
	if p.whole && p.made == 0 && len(p.now) > 0 {
		u.served++
	}
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	g, ps := u.gen, &partSet{elems: p.now, file: file}
	g.dropPart(p.key)
	if p.whole {
		g.dropLoad(p.key)
	}
	size := ps.size() // under the lock: a later stage attaching to a carried element grows it
	if !u.m.admit(g, size, false) {
		return
	}
	g.parts[p.key] = ps
	g.bytes += size
}

// dropPart deletes the elements kept for the load k.
func (g *memoGen) dropPart(k loadKey) {
	if ps := g.parts[k]; ps != nil {
		g.bytes -= ps.size()
		delete(g.parts, k)
	}
}

// dropLoad deletes the record kept for the load k.
func (g *memoGen) dropLoad(k loadKey) {
	if le := g.loads[k]; le != nil {
		g.bytes -= le.en.size
		delete(g.loads, k)
	}
}

// dropParts deletes the elements of the loads of files no longer alive.
func (g *memoGen) dropParts(alive func(*syntax.File) bool) {
	for k, ps := range g.parts { //canon:unordered deleting the dead ones, in any order
		if !alive(ps.file) {
			g.dropPart(k)
		}
	}
}

// size is the bytes of ps's elements, what later stages keep on them included.
func (ps *partSet) size() int {
	n := 0
	for _, en := range ps.elems { //canon:unordered a sum
		n += en.size
	}
	return n
}

// holdsPart reports g keeping en as an element of its load; m.mu is held.
func (g *memoGen) holdsPart(en *memoEntry) bool {
	ps := g.parts[en.part.load]
	return ps != nil && ps.elems[en.part.src] == en
}

// LoadsServed is the loads e ran again whose every element it served from its memo: a hook for tests.
func (e *Evaluator) LoadsServed() int {
	if e.memo == nil {
		return 0
	}
	return e.memo.served
}

// PartsReplayed is the load.dir elements e served from its memo: a hook for tests.
func (e *Evaluator) PartsReplayed() int {
	if e.memo == nil {
		return 0
	}
	return e.memo.parted.hits
}
