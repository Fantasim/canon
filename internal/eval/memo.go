package eval

import (
	"slices"
	"strings"
	"sync"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Memo keeps each `entry` declaration's evaluation, and each load's, for the evaluators of later
// snapshots, which replay it: its steps, findings, marks and value. It keeps the stores of the
// last few epochs, one per Recheck lineage. It is safe for concurrent use.
type Memo struct {
	mu        sync.Mutex
	gens      []*memoGen // the latest epoch used first
	forgotten []uint64   // the latest epochs forgotten, whose stores are never made again
}

// memoGen is what a Memo keeps for one type-check epoch: the entries and loads, the collections
// an evaluator makes itself, shared so that the entries replayed name the same ones, and what
// each file adds to an evaluator's indexes.
type memoGen struct {
	epoch   uint64
	entries map[memoKey]*memoEntry
	loads   map[loadKey]*loadEntry
	colls   map[collKey]*types.Collection
	indexed map[*syntax.File]fileIndex
	refined map[*syntax.File][]typeAt
	bytes   int
}

// liveness tells the declarations and files of the program an evaluator checks.
type liveness struct {
	decl func(*syntax.EntryDecl) bool
	file func(*syntax.File) bool
}

// memoKey is what an entry's evaluation depends on besides the values it reads and the code
// it runs: its declaration (a changed file brings new nodes), the let's type and collection,
// the active layers, and whether the let was already tainted.
type memoKey struct {
	decl    *syntax.EntryDecl
	t       types.Type
	coll    *types.Collection
	layers  string
	tainted bool
}

// memoEntry is one entry's evaluation: the steps it charged around each value it read, the
// frames and code it needed, its findings, and its record with its marks (no root when it
// failed).
type memoEntry struct {
	reads []memoRead
	tail  int64
	need  int
	files []*syntax.File
	found []memoFinding
	kept  memoGraph
	size  int
}

// memoRead is a top-level value an entry read: the steps charged before it, the frames above
// the entry's own when it was forced, and the fingerprint of the value read.
type memoRead struct {
	steps           int64
	root            Root
	depth, implicit int
	fp              fingerprint
}

// memoFinding is a finding an entry reported, the package whose bag took it, and how many
// values the entry had read then: a replay stopped at a read reports those before it.
type memoFinding struct {
	pkg  string
	b    *diag.Builder
	read int
}

// memoUse is an evaluator's use of a memo: the epoch's store, the entry being recorded, and
// what it learnt of the values its entries read.
type memoUse struct {
	m      *Memo
	gen    *memoGen
	layers string
	trace  *entryTrace
	reads  map[value.Value]*readInfo
	stats  memoStats
	loaded memoStats // the same counts for loads
}

// memoStats counts, for tests, the entries replayed, recorded and evaluated without a record.
type memoStats struct {
	hits, stored, unkept int
}

// NewMemo is an empty memo.
func NewMemo() *Memo {
	return &Memo{}
}

// UseMemo makes e replay and record its entries and loads in m's store of epoch, dropping what the
// program no longer declares; a forgotten epoch runs without. Epoch rule: a new epoch for every
// full check.Check, the same one only along one Session.Recheck lineage.
func (e *Evaluator) UseMemo(m *Memo, epoch uint64) {
	if m == nil || e.prog == nil {
		return
	}
	x := e.index
	alive := liveness{
		decl: func(d *syntax.EntryDecl) bool { return x.decls[d] != nil },
		file: func(f *syntax.File) bool { _, ok := x.pkg[f]; return ok },
	}
	g := m.begin(epoch, alive)
	if g == nil {
		return
	}
	e.memo = &memoUse{
		m: m, gen: g,
		layers: strings.Join(e.opt.Layers, layerSep),
		reads:  map[value.Value]*readInfo{},
	}
	x.written.memo = e.memo
}

// begin is the store of epoch, made the most recent, emptied of what belongs to declarations and
// files gone; nil for an epoch forgotten (a stale Recheck, a late UseMemo).
func (m *Memo) begin(epoch uint64, alive liveness) *memoGen {
	m.mu.Lock()
	defer m.mu.Unlock()
	if slices.Contains(m.forgotten, epoch) {
		return nil
	}
	i := slices.IndexFunc(m.gens, func(g *memoGen) bool { return g.epoch == epoch })
	if i < 0 {
		g := &memoGen{
			epoch: epoch, entries: map[memoKey]*memoEntry{}, loads: map[loadKey]*loadEntry{},
			colls:   map[collKey]*types.Collection{},
			indexed: map[*syntax.File]fileIndex{}, refined: map[*syntax.File][]typeAt{},
		}
		m.gens = append([]*memoGen{g}, m.gens[:min(len(m.gens), memoEpochs-1)]...)
		return g
	}
	g := m.gens[i]
	m.gens = slices.Insert(slices.Delete(m.gens, i, i+1), 0, g)
	for k, en := range g.entries { //canon:unordered deleting the dead ones, in any order
		if !alive.decl(k.decl) {
			g.bytes -= en.size
			delete(g.entries, k)
		}
	}
	g.dropLoads(alive.file)
	dropFiles(g.indexed, alive.file)
	dropFiles(g.refined, alive.file)
	return g
}

// Forget drops the store of epoch, which no lineage continues, and makes none for it again; an
// evaluator using it keeps it (log-2026-09-29 M4 P3-r).
func (m *Memo) Forget(epoch uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gens = slices.DeleteFunc(m.gens, func(g *memoGen) bool { return g.epoch == epoch })
	if !slices.Contains(m.forgotten, epoch) {
		m.forgotten = append(m.forgotten[max(len(m.forgotten)-memoForgotten+1, 0):], epoch)
	}
}

// Kept reports that m holds a store for epoch.
func (m *Memo) Kept(epoch uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.ContainsFunc(m.gens, func(g *memoGen) bool { return g.epoch == epoch })
}

// dropFiles deletes the files of cache no longer alive.
func dropFiles[T any](cache map[*syntax.File]T, alive func(*syntax.File) bool) {
	for f := range cache { //canon:unordered deleting the dead ones, in any order
		if !alive(f) {
			delete(cache, f)
		}
	}
}

// filesIndexed is an epoch's cache of each file's fileIndex, for cachedFile.
func filesIndexed(g *memoGen) map[*syntax.File]fileIndex {
	return g.indexed
}

// filesRefined is an epoch's cache of each file's written types, for cachedFile.
func filesRefined(g *memoGen) map[*syntax.File][]typeAt {
	return g.refined
}

// cachedFile is build(f), kept by u's epoch in the cache of's: an unchanged file is the same
// tree, and build reads nothing else. Without a memo, build(f).
func cachedFile[T any](u *memoUse, of func(*memoGen) map[*syntax.File]T, f *syntax.File, build func(*syntax.File) T) T {
	if u == nil {
		return build(f)
	}
	u.m.mu.Lock()
	v, ok := of(u.gen)[f]
	u.m.mu.Unlock()
	if ok {
		return v
	}
	v = build(f)
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	of(u.gen)[f] = v
	return v
}

// lookup is the entry kept under k, nil for none.
func (u *memoUse) lookup(k memoKey) *memoEntry {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	return u.gen.entries[k]
}

// store keeps en under k; a memo past its bound on entries or bytes forgets them all first.
func (u *memoUse) store(k memoKey, en *memoEntry) {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	g := u.gen
	if old := g.entries[k]; old != nil {
		g.bytes -= old.size
		delete(g.entries, k)
	}
	if !u.m.admit(g, en.size, len(g.entries) >= memoCap) {
		u.stats.unkept++
		return
	}
	g.entries[k] = en
	g.bytes += en.size
	u.stats.stored++
}

// collection is the epoch's collection of c's key and element, c the first time: every evaluator
// using the memo names the same one, as the entries it replays do.
func (e *Evaluator) collection(c *types.Collection) *types.Collection {
	u := e.memo
	if u == nil {
		return c
	}
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	if kept := u.gen.colls[keyOf(c)]; kept != nil && kept.Elem == c.Elem && kept.KeyedBy == c.KeyedBy {
		return kept
	}
	u.gen.colls[keyOf(c)] = c
	return c
}
