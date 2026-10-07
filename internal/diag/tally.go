package diag

import "slices"

// tally counts a bag's distinct findings as they come, whatever its limit.
type tally struct {
	n      int // the findings tallied
	seen   map[dupKey]struct{}
	counts Truncation // only its Errors and Warnings
}

// ErrorCount is Summary().Errors without the view, at the cost of the findings new since the last ask.
func (b *Bag) ErrorCount() int {
	return b.distinct().errors
}

// tallied is the counts of one read of the bag: no more findings than its limit drops none.
type tallied struct {
	errors, warnings int
	holdsAll         bool
}

// distinct is the bag's error and warning counts, the dropped findings included (API.md F7).
func (b *Bag) distinct() tallied {
	b.tallyMu.Lock()
	defer b.tallyMu.Unlock()
	b.mu.Lock()
	all, limit := slices.Clip(b.findings), b.max
	b.mu.Unlock()
	t := &b.tally
	if t.seen == nil {
		t.seen = map[dupKey]struct{}{}
	}
	for i := t.n; i < len(all); i++ {
		k := keyOf(b.files, &all[i])
		if _, dup := t.seen[k]; !dup {
			t.seen[k] = struct{}{}
			t.counts.add(k.severity)
		}
	}
	t.n = len(all)
	return tallied{errors: t.counts.Errors, warnings: t.counts.Warnings, holdsAll: len(all) <= limit}
}
