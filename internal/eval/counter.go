package eval

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
)

// counter is the step budget of each package and what it was spent on (EVALUATION.md §12.2).
type counter struct {
	budget    int64
	steps     int64            // every package's
	per       map[string]int64 // each package's
	spent     map[charge]int64
	order     []charge
	out       map[string]*diag.Builder // each spent budget's E4401, nil for a vector's cut
	reported  []string                 // the package of the bag of each E4401, in order
	whole     bool                     // one budget for every package: a vector's cap
	exhausted bool                     // cancelled, or a vector cut short
}

// newCounter is a counter of opt's budget per package, nothing spent.
func newCounter(opt Options) *counter {
	budget := opt.Budget
	if budget <= 0 {
		budget = DefaultBudget
	}
	return &counter{budget: budget, per: map[string]int64{}, spent: map[charge]int64{}, out: map[string]*diag.Builder{}}
}

// key is the counter pkg spends: its own, or the one budget of a whole counter.
func (c *counter) key(pkg string) string {
	if c.whole {
		return ""
	}
	return pkg
}

// pay spends n steps on ch; true once they reach its package's budget.
func (c *counter) pay(ch charge, n int64) bool {
	if c.spent[ch] == 0 {
		c.order = append(c.order, ch)
	}
	k := c.key(ch.pkg)
	c.steps += n
	c.per[k] += n
	c.spent[ch] += n
	return c.per[k] >= c.budget
}

// takeBack returns the steps ch was charged past to, to its package's budget.
func (c *counter) takeBack(ch charge, to int64) {
	n := c.spent[ch] - to
	c.steps -= n
	c.per[c.key(ch.pkg)] -= n
	c.spent[ch] = to
}

// fits reports n more steps of pkg within its budget: spending them leaves one at least.
func (c *counter) fits(pkg string, n int64) bool {
	return c.per[c.key(pkg)]+n < c.budget
}

// spentOut reports pkg's budget spent: E4401 was reported for it.
func (c *counter) spentOut(pkg string) bool {
	_, ok := c.out[c.key(pkg)]
	return ok
}

// stopped reports that some evaluation stopped: a budget spent, a cancellation or a cut.
func (c *counter) stopped() bool {
	return c.exhausted || len(c.out) > 0
}

// heaviest is the charge of pkg's budget charged the most steps, the first charged on a tie.
func (c *counter) heaviest(pkg string) charge {
	var heavy charge
	k, found := c.key(pkg), false
	for _, ch := range c.order {
		if c.key(ch.pkg) == k && (!found || c.spent[ch] > c.spent[heavy]) {
			heavy, found = ch, true
		}
	}
	return heavy
}

// halted reports that pkg's work does not run: stopped, or pkg's budget spent outside phase 8 (EVALUATION.md §2.1).
func (e *Evaluator) halted(pkg string) bool {
	return e.exhausted || !e.late.free && e.spentOut(pkg)
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

// Replay is a new folder, on counters of its own, that made fs's folds again into bags, each
// seeing Broken as it first did: its counter, charges and constants are the original folder's at
// that point (log-2026-09-29 M4 B2-r). Its findings are the caller's to discard.
func (fs Folds) Replay(ctx context.Context, bags check.Bags) check.Folder {
	f := NewFolder(bags, fs.opt).(*folder)
	for _, c := range fs.calls {
		f.fold(ctx, foldCall{owner: c.owner, e: c.e, info: c.info, broken: c.broken, replay: true})
	}
	return f
}
