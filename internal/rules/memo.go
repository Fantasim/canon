package rules

import (
	"context"
	"sync"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// Memo keeps stage C in each entry of a top-level table for later snapshots: an entry of the same
// token (eval.Evaluator.EntryToken) and marks has its check runs replayed, their reads compared,
// instead of being traversed. It is safe for concurrent use.
type Memo struct {
	mu       sync.Mutex
	gen      *memoGen
	replayed int
}

// Replayed is how many entries the runners using m replayed, for tests and NFR-01 evidence.
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

// entryKey is an entry's evaluation and its top-level value.
type entryKey struct {
	token any
	root  eval.Root
}

// entryKept is stage C in one entry: the places of its invalid values, each check run in order,
// and those that failed.
type entryKept struct {
	marks  []int
	runs   []*eval.CheckTrace
	failed []failedRun
}

// failedRun is a run that failed: its check, trace, instance (by visiting order in the entry) and path.
type failedRun struct {
	c        *syntax.CheckDecl
	run      int
	instance int
	at       *verify.Path
}

// entryRec is the entry being recorded: the instances visited so far, what it kept, and void:
// a run a replay cannot reproduce.
type entryRec struct {
	visited  int
	instance int
	kept     entryKept
	void     bool
}

// memoEvaluator is what an Evaluator also serves for the memo: marks, tokens, traced and
// replayed runs, each of those told with its instance as Run tells its own (VIEWMODEL.md J15).
type memoEvaluator interface {
	marks
	EntryToken(rec *value.Record) (any, bool)
	RunTraced(ctx context.Context, c *syntax.CheckDecl, self value.Value) (eval.CheckRun, *eval.CheckTrace)
	ReplayChecks(ctx context.Context, runs []*eval.CheckTrace) bool
	Ran(c *syntax.CheckDecl, self value.Value, run Run)
}

// memoUse is a runner's use of a memo: the epoch's store, the evaluator, and which top-level
// values no other value traversed can hold a part of.
type memoUse struct {
	m     *Memo
	gen   *memoGen
	ev    memoEvaluator
	alone func(eval.Root) bool
}

// NewMemo is an empty memo.
func NewMemo() *Memo {
	return &Memo{}
}

// UseMemo makes r replay and record the entries of the top-level tables alone tells, which no
// other top-level value traversed holds a part of, when its evaluator serves the memo. Epoch
// rule: as eval.Evaluator.UseMemo; a new epoch drops all m kept.
func (r *Runner) UseMemo(m *Memo, epoch uint64, alone func(eval.Root) bool) {
	ev, ok := r.ev.(memoEvaluator)
	if m == nil || !ok {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gen == nil || m.gen.epoch != epoch {
		m.gen = &memoGen{epoch: epoch, entries: map[entryKey]*entryKept{}}
	}
	r.memo = &memoUse{m: m, gen: m.gen, ev: ev, alone: alone}
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
