package eval

import (
	"container/heap"
	"slices"

	"github.com/fantasim/canonlang/internal/value"
)

// judgeOrder is recs, each literal before the literals it holds, otherwise in recording order (DECISIONS 446).
func (k *keptLits) judgeOrder(recs []keptRec) []keptRec {
	at := make(map[value.Value]int, len(recs))
	for i, rec := range recs {
		at[rec.literal()] = i
	}
	held := make([][]int, len(recs))
	holders := make([]int, len(recs))
	nested := false
	for i, rec := range recs {
		held[i] = k.holds(rec, at)
		for _, j := range held[i] {
			holders[j]++
			nested = true
		}
	}
	if !nested {
		return recs
	}
	return ordered(recs, held, holders)
}

// ordered is recs with each one after its holders, the first recorded first among those free.
func ordered(recs []keptRec, held [][]int, holders []int) []keptRec {
	free := &indexHeap{}
	for i, n := range holders {
		if n == 0 {
			heap.Push(free, i)
		}
	}
	out := make([]keptRec, 0, len(recs))
	done := make([]bool, len(recs))
	for free.Len() > 0 {
		i := heap.Pop(free).(int)
		out = append(out, recs[i])
		done[i] = true
		for _, j := range held[i] {
			if holders[j]--; holders[j] == 0 {
				heap.Push(free, j)
			}
		}
	}
	// A cycle no literal can form today would leave some unordered: they follow in recording order, never dropped.
	for i, rec := range recs {
		if !done[i] {
			out = append(out, rec)
		}
	}
	return out
}

// literal is rec's literal value: its map or its record.
func (rec keptRec) literal() value.Value {
	if rec.r != nil {
		return rec.r
	}
	return rec.m
}

// holds is the literals of at that rec's literal holds directly, a copy standing for its literal,
// in recording order: its parts walked down to the first literal on each path.
func (k *keptLits) holds(rec keptRec, at map[value.Value]int) []int {
	var out []int
	seen := map[value.Value]bool{}
	stack := components(rec.literal())
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[x] {
			continue
		}
		seen[x] = true
		if j, ok := at[k.standsFor(x)]; ok {
			out = append(out, j)
			continue
		}
		stack = append(stack, components(x)...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// standsFor is the record literal x is a copy of, else x.
func (k *keptLits) standsFor(x value.Value) value.Value {
	if rec, ok := x.(*value.Record); ok && k.literal[rec] != nil {
		return k.literal[rec]
	}
	return x
}

// indexHeap is a min-heap of record indexes (container/heap).
type indexHeap []int

func (h indexHeap) Len() int           { return len(h) }
func (h indexHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h indexHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

// Push adds x, an index.
func (h *indexHeap) Push(x any) { *h = append(*h, x.(int)) }

// Pop takes the last index.
func (h *indexHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}
