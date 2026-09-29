package build

import (
	"context"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
)

// Cause is the errors that poisoned root: its own, or the first poisoned value's it read, in any
// package (API.md R6, S11). A run that replayed the memo logged none: phases 3 to 7 then run
// again with causes logged, outside the analysis's lock; a cancelled run is not kept.
func (a *Analysis) Cause(ctx context.Context, root eval.Root) ([]diag.Finding, error) {
	a.mu.Lock()
	ev := a.causes
	if !a.r.memoized {
		ev = a.r.ev
	}
	a.mu.Unlock()
	if ev == nil {
		run, err := a.r.causeRun(ctx)
		if err != nil {
			return nil, err
		}
		a.mu.Lock()
		if a.causes == nil {
			a.causes = run
		}
		ev = a.causes
		a.mu.Unlock()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return ev.PoisonCause(root).Findings, nil
}

// causeRun is phases 3 to 7 again over r's checked program, with causes logged, without the
// memo, into a copy of the snapshot's bags: the log a cold analysis keeps; ctx.Err() when
// cancelled (log-2026-09-29 M4 U8-r).
func (r *run) causeRun(ctx context.Context) (*eval.Evaluator, error) {
	s := *r.s
	s.own, s.bags = diag.NewBag(s.set, ""), map[string]*diag.Bag{}
	w := &run{p: r.p, s: &s, selected: r.selected, loaded: r.loaded, prog: r.prog, opt: r.opt, texts: r.texts, causes: true}
	w.bags = s.bagsOf(w.loaded)
	w.fold = eval.NewFolder(w.bags, w.opt)
	err := w.evaluate(ctx)
	if cerr := ctx.Err(); cerr != nil {
		return nil, cerr
	}
	_ = err // a failure of the compiler: the analysis reported it already
	return w.ev, nil
}
