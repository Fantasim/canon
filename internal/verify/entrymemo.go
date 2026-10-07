package verify

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// entryMemo is what an Evaluator also serves for the memo: which evaluation an entry's record
// replays, and a slot on that evaluation for what verifying the record found.
type entryMemo interface {
	EntryToken(rec *value.Record) (any, bool)
	TokenOwner(token any) (eval.Root, bool)
	Attach(token any, stage eval.Stage, v any, nodes int) bool
	Attached(token any, stage eval.Stage) any
}

// entryKey is what an entry's verification depends on besides its evaluation and its reads: its
// top-level value, the type it is verified against, whether it is retired, and the last segment
// of its path, which a list element's findings carry (API.md P8).
type entryKey struct {
	root    eval.Root
	elem    types.Type
	retired bool
	seg     string
	form    segForm
}

// entryKept is an entry's verification: its key, what it read, in order, what it reported, and
// the places, in nodeOrder, of the values it marked invalid.
type entryKept struct {
	key   entryKey
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
	look    lookup
}

// entryRec is an entry's verification being recorded; void, one a replay could not reproduce.
type entryRec struct {
	reads  []entryRead
	found  []*diag.Builder
	marked []value.Value
	void   bool
}

// UseMemo makes v replay and record the entries of its top-level tables on their evaluations,
// which its evaluator keeps in its memo (eval.Evaluator.Attach).
func (v *Verifier) UseMemo() {
	if m, ok := v.ev.(entryMemo); ok {
		v.memo = m
	}
}

// Replayed is how many entries v replayed, for tests and NFR-01 evidence.
func (v *Verifier) Replayed() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.replayed
}

// hit counts an entry replayed.
func (v *Verifier) hit() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.replayed++
}

// nodes is about how many values, reads and findings k keeps.
func (k *entryKept) nodes() int {
	return 1 + len(k.reads) + len(k.found) + len(k.marks)
}

// voidRec voids the entry being recorded, if any: its verification did what a replay cannot.
func (w *walker) voidRec() {
	if w.rec != nil {
		w.rec.void = true
	}
}
