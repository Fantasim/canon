package verify

import (
	"sync"

	"github.com/fantasim/canonlang/internal/value"
)

// targets is each ref target's entries by key, once per settled collection value (EVALUATION.md §4.5; DECISIONS 515).
type targets struct {
	mu     sync.Mutex
	byColl map[value.Value]map[value.Key]*value.Record
}

func newTargets() *targets {
	return &targets{byColl: map[value.Value]map[value.Key]*value.Record{}}
}

// Apart is ix whose verifiers index ref targets on their own: for an evaluator that ends before
// ix's run does (a throwaway host's, a world's, a vector's), so what its values index dies with it.
func (ix *Index) Apart() *Index {
	return &Index{src: ix.src, declared: ix.declared, targets: newTargets()}
}

// Indexed is how many top-level collections ix's verifiers indexed, for tests and NFR-01 evidence.
func (ix *Index) Indexed() int {
	return ix.targets.len()
}

func (t *targets) len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.byColl)
}

// of maps each key of a table or keyed list to its first entry, once per collection value.
func (t *targets) of(coll value.Value) map[value.Key]*value.Record {
	if coll == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if idx, ok := t.byColl[coll]; ok {
		return idx
	}
	idx := map[value.Key]*value.Record{}
	for _, e := range entriesOf(coll) {
		if _, seen := idx[e.Ident.Key]; !seen {
			idx[e.Ident.Key] = e
		}
	}
	t.byColl[coll] = idx
	return idx
}
