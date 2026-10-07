package build

import (
	"github.com/fantasim/canonlang/internal/eval"
)

// Covers reports that a gives every package's values (log-2026-09-29 M4 P14-r, DECISIONS 244):
// all, every package's analysis where the packages a leaves out read what they read on a's
// snapshot and none of a's, spent less than the budget, and so do its steps on those plus a's.
func Covers(all, a *Analysis) bool {
	if len(all.r.selected) != len(all.r.s.units) || all.r.opt.Budget != a.r.opt.Budget {
		return false
	}
	var total, left int64
	for _, c := range all.charges() {
		total += c.Steps
		if !a.r.selects(c.Pkg) {
			left += c.Steps
		}
	}
	for _, c := range a.charges() {
		left += c.Steps
	}
	budget := a.r.opt.Budget
	if budget <= 0 {
		budget = eval.DefaultBudget
	}
	return total < budget && left < budget
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
