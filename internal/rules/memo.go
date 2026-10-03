package rules

import (
	"context"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// entryKept is stage C in one entry of a top-level value: that value, the entry's segment of
// its path (a list element's index or key), the places of the entry's invalid values, each check
// run in order, and those that failed.
type entryKept struct {
	root   eval.Root
	seg    seg
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

// memoEvaluator is what an Evaluator also serves for the memo: marks, tokens and their slots,
// traced and replayed runs, each told with its instance as Run tells its own (VIEWMODEL.md J15).
type memoEvaluator interface {
	marks
	EntryToken(rec *value.Record) (any, bool)
	TokenOwner(token any) (eval.Root, bool)
	Attach(token any, stage eval.Stage, v any, nodes int) bool
	Attached(token any, stage eval.Stage) any
	RunTraced(ctx context.Context, c *syntax.CheckDecl, self value.Value, path string) (eval.CheckRun, *eval.CheckTrace)
	ReplayChecks(ctx context.Context, runs []*eval.CheckTrace) bool
}

// memoUse is a runner's use of the memo: the evaluator keeping it, and which top-level values no
// other value traversed can hold a part of.
type memoUse struct {
	ev       memoEvaluator
	alone    func(eval.Root) bool
	replayed int
}

// UseMemo makes r replay and record, on their evaluations, the entries of the top-level tables
// alone tells no other value traversed holds a part of, when its evaluator keeps a memo.
func (r *Runner) UseMemo(alone func(eval.Root) bool) {
	if ev, ok := r.ev.(memoEvaluator); ok {
		r.memo = &memoUse{ev: ev, alone: alone}
	}
}

// Replayed is how many entries r replayed, for tests and NFR-01 evidence.
func (r *Runner) Replayed() int {
	if r.memo == nil {
		return 0
	}
	return r.memo.replayed
}

// nodes is about how many runs, reads and findings k keeps.
func (k *entryKept) nodes() int {
	n := 1 + len(k.marks) + len(k.failed)
	for _, run := range k.runs {
		n += run.Nodes()
	}
	return n
}
