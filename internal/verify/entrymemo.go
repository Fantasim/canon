package verify

import (
	"sync"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Memo keeps what verifying each entry of a top-level table found, for later snapshots: an entry
// of the same token (eval.Evaluator.EntryToken) asks again only what its verification read, the
// ref targets and assets, and reports what it found. It is safe for concurrent use.
type Memo struct {
	mu       sync.Mutex
	gen      *memoGen
	replayed int
}

// Replayed is how many entries the verifiers using m replayed, for tests and NFR-01 evidence.
func (m *Memo) Replayed() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.replayed
}

// memoGen is what a Memo keeps for one type-check epoch.
type memoGen struct {
	epoch   uint64
	entries map[entryKey]*entryKept
}

// tokens is what an Evaluator may also tell: which evaluation an entry's record replays.
type tokens interface {
	EntryToken(rec *value.Record) (any, bool)
}

// entryKey is what an entry's verification depends on besides its reads: its evaluation, its
// top-level value, the type it is verified against and whether it is retired.
type entryKey struct {
	token   any
	root    eval.Root
	elem    types.Type
	retired bool
}

// entryKept is an entry's verification: what it read, in order, what it reported, and the
// places, in nodeOrder, of the values it marked invalid.
type entryKept struct {
	reads []entryRead
	found []*diag.Builder
	marks []int
}

// entryRead is a ref's target looked up, or an asset looked up, with what it gave.
type entryRead struct {
	coll    *types.Collection // a ref's target collection; nil for an asset
	key     value.Key
	asset   *types.AssetSpec
	name    string
	how     reach
	found   bool
	retired bool
	display string
}

// entryRec is an entry's verification being recorded; void, one a replay could not reproduce.
type entryRec struct {
	reads  []entryRead
	found  []*diag.Builder
	marked []value.Value
	void   bool
}

// memoUse is a verifier's use of a memo: the epoch's store and the evaluator's tokens.
type memoUse struct {
	m      *Memo
	gen    *memoGen
	tokens tokens
}

// NewMemo is an empty memo.
func NewMemo() *Memo {
	return &Memo{}
}

// UseMemo makes v replay and record the entries of its top-level tables in m, when its
// evaluator tells their tokens. Epoch rule: as eval.Evaluator.UseMemo; a new epoch drops all m kept.
func (v *Verifier) UseMemo(m *Memo, epoch uint64) {
	t, ok := v.ev.(tokens)
	if m == nil || !ok {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gen == nil || m.gen.epoch != epoch {
		m.gen = &memoGen{epoch: epoch, entries: map[entryKey]*entryKept{}}
	}
	v.memo = &memoUse{m: m, gen: m.gen, tokens: t}
}

func (u *memoUse) lookup(k entryKey) *entryKept {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	return u.gen.entries[k]
}

// hit counts an entry replayed.
func (u *memoUse) hit() {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	u.m.replayed++
}

// store keeps en under k; a memo past its bound forgets every entry first.
func (u *memoUse) store(k entryKey, en *entryKept) {
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	if len(u.gen.entries) >= memoEntries {
		u.gen.entries = map[entryKey]*entryKept{}
	}
	u.gen.entries[k] = en
}

// voidRec voids the entry being recorded, if any: its verification did what a replay cannot.
func (w *walker) voidRec() {
	if w.rec != nil {
		w.rec.void = true
	}
}
