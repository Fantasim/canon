package build

import (
	"github.com/fantasim/canonlang/internal/eval"
)

// Covers reports that a gives every package's values: all analyses every package, those a leaves
// out read none of a's, and no package's budget runs out, one a selects spending what it spends in
// a (what reads it is selected too), any other at most its steps in all plus those in a.
func Covers(all, a *Analysis) bool {
	if len(all.r.selected) != len(all.r.s.units) || all.r.opt.Budget != a.r.opt.Budget {
		return false
	}
	need := map[string]int64{}
	for _, c := range all.charges() {
		if !a.r.selects(c.Pkg) {
			need[c.Pkg] += c.Steps
		}
	}
	for _, c := range a.charges() {
		need[c.Pkg] += c.Steps
	}
	budget := a.r.opt.Budget
	if budget <= 0 {
		budget = eval.DefaultBudget
	}
	for _, n := range need { //canon:unordered a predicate over every package
		if n >= budget {
			return false
		}
	}
	return true
}

// charges is every root's steps a's run spent, phase 2's folds included.
func (a *Analysis) charges() []eval.Charge {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.r.ev.Charged()
}

// RealPaths are names resolved through the links of the file system a's snapshot reads, as
// Static.Readers compares them (API.md S12; log-2026-09-29 M4 P14-r4).
func (a *Analysis) RealPaths(names ...string) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	real := newRealPaths(a.r.p.fs)
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = real.file(n)
	}
	return out
}
