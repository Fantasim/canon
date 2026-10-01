package eval

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
)

// counter is a step budget and what it was spent on; an Evaluator embeds the one it spends.
type counter struct {
	budget    int64
	steps     int64
	spent     map[charge]int64
	order     []charge
	exhausted bool
}

// newCounter is a counter of opt's budget, nothing spent.
func newCounter(opt Options) *counter {
	budget := opt.Budget
	if budget <= 0 {
		budget = DefaultBudget
	}
	return &counter{budget: budget, spent: map[charge]int64{}}
}

// Folds is the folds one folder made up to a point, in order (FoldsOf).
type Folds struct {
	calls []foldCall
	reads []Root
	opt   Options
}

// FoldsOf is the folds f, NewFolder's, has made so far; the zero Folds for another Folder.
func FoldsOf(f check.Folder) Folds {
	x, ok := f.(*folder)
	if !ok {
		return Folds{}
	}
	return Folds{calls: slices.Clip(x.calls), reads: slices.Clip(x.reads.roots), opt: x.opt}
}

// Reads is every constant fs's folds read, once each, in fold order: stage A's item 3 (EVALUATION.md §2.1).
func (fs Folds) Reads() []Root {
	return slices.Clone(fs.reads)
}

// Replay is a new folder, on a counter of its own, that made fs's folds again into bags, each
// seeing Broken as it first did: its counter, charges and constants are the original folder's at
// that point (log-2026-09-29 M4 B2-r). Its findings are the caller's to discard.
func (fs Folds) Replay(ctx context.Context, bags check.Bags) check.Folder {
	f := NewFolder(bags, fs.opt).(*folder)
	for _, c := range fs.calls {
		f.fold(ctx, foldCall{owner: c.owner, e: c.e, info: c.info, broken: c.broken, replay: true})
	}
	return f
}
