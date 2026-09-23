package lock

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// Update adds the new facts and retirements a build with no error records (LOCK.md §5).
func (f *File) Update(s *Sources) (bool, error) {
	changed := false
	for _, fact := range s.facts.facts {
		fact.Span = source.Span{}
		c, err := f.Add(fact)
		if err != nil {
			return changed, err
		}
		changed = changed || c
	}
	return changed, nil
}

// Pending reports W6006 for the values the lock lacks and returns their count; retirements
// and facts that conflict with the lock are not counted.
func (f *File) Pending(s *Sources, at source.Span, bag *diag.Bag) int {
	lockAll, n := f.byColl(), 0
	for c, facts := range s.facts.byColl() { //canon:unordered a count
		known, values := holders(lockAll[c]), map[Value]bool{}
		for _, l := range lockAll[c] {
			values[l.Value] = true
		}
		for _, fact := range facts {
			_, heldBefore := known[fact.Holder]
			if !s.skipped(c) && !heldBefore && (c.kind == KindTable || !values[fact.Value]) {
				n++
			}
		}
	}
	switch {
	case n == 1:
		diag.W6006.AtOne(at).Report(bag)
	case n > 1:
		diag.W6006.AtMany(at, int64(n)).Report(bag)
	}
	return n
}
