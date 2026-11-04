package build

import (
	"math"

	"github.com/fantasim/canonlang/internal/diag"
)

// OwnWhole is Own's findings in the source language, no bag cut at its limit (API.md F7), for an
// edit's inactive-layer merge (E18). Only on an unpublished analysis, private to the write
// transaction: its lift and restore of each bag's cut is not guarded against concurrent Own.
func (a *Analysis) OwnWhole() []diag.Finding {
	a.mu.Lock()
	defer a.mu.Unlock()
	r := a.r
	out := uncut(r.s.own, diag.DefaultMaxFindings) // the project's own bag keeps diag's default (NewBag)
	bags := make([]*diag.Bag, 0, len(r.selected)+len(r.orphans))
	for _, u := range r.selected {
		bags = append(bags, r.s.bag(u.Name))
	}
	for _, b := range append(bags, r.orphanBags()...) {
		out = append(out, uncut(b, r.p.findingLimit())...)
	}
	return out
}

// uncut is b's findings with no cut, the limit lifted for this read only and put back at limit,
// as unreported does.
func uncut(b *diag.Bag, limit int) []diag.Finding {
	b.Truncate(math.MaxInt)
	defer b.Truncate(limit)
	return b.Findings()
}
