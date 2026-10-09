package eval

import (
	"slices"
	"testing"
)

const counterBudget = 10

// counterOp is one step of a counter's life: pay n on ch, or take ch back to n, dropping it
// from the order when that leaves it nothing, as an undone Savepoint does.
type counterOp struct {
	ch   charge
	n    int64
	back bool
}

// mapCounter is the counter as plain maps, every step a map access: the reference the tallies
// must agree with.
type mapCounter struct {
	whole bool
	steps int64
	per   map[string]int64
	spent map[charge]int64
	order []charge
}

func (m *mapCounter) key(pkg string) string {
	if m.whole {
		return ""
	}
	return pkg
}

func (m *mapCounter) apply(op counterOp) bool {
	if op.back {
		n := m.spent[op.ch] - op.n
		m.steps -= n
		m.per[m.key(op.ch.pkg)] -= n
		m.spent[op.ch] = op.n
		if op.n == 0 {
			m.order = slices.DeleteFunc(m.order, func(c charge) bool { return c == op.ch })
		}
		return false
	}
	if m.spent[op.ch] == 0 {
		m.order = append(m.order, op.ch)
	}
	m.steps += op.n
	m.per[m.key(op.ch.pkg)] += op.n
	m.spent[op.ch] += op.n
	return m.per[m.key(op.ch.pkg)] >= counterBudget
}

func applyTo(c *counter, op counterOp) bool {
	if !op.back {
		return c.pay(op.ch, op.n)
	}
	c.takeBack(op.ch, op.n)
	if op.n == 0 {
		c.order = slices.DeleteFunc(c.order, func(x charge) bool { return x == op.ch })
	}
	return false
}

// sameCounts fails t when c's steps, order or tallies of chs differ from m's after op i.
func sameCounts(t *testing.T, c *counter, m *mapCounter, i int, chs []charge) {
	t.Helper()
	if c.steps != m.steps || !slices.Equal(c.order, m.order) {
		t.Fatalf("whole %t, op %d: steps %d order %v, want %d %v", m.whole, i, c.steps, c.order, m.steps, m.order)
	}
	for _, ch := range chs {
		if c.spentOn(ch) != m.spent[ch] || c.perOf(ch.pkg) != m.per[m.key(ch.pkg)] {
			t.Fatalf("whole %t, op %d, %s: spent %d per %d, want %d %d", m.whole, i, ch.name, c.spentOn(ch), c.perOf(ch.pkg), m.spent[ch], m.per[m.key(ch.pkg)])
		}
	}
}

// EVALUATION.md §12.2: tallies agree with plain maps across interleaved charges, a take-back to none and a whole counter.
func TestCounterTallies(t *testing.T) {
	a, b, q := charge{pkg: "p", name: "a"}, charge{pkg: "p", name: "b"}, charge{pkg: "q", name: "c"}
	ops := []counterOp{
		{ch: a, n: 3}, {ch: b, n: 1}, {ch: a, n: 2}, {ch: q, n: 4},
		{ch: a, n: 0, back: true}, {ch: b, n: 2}, {ch: a, n: 1}, {ch: q, n: 2, back: true},
		{ch: a, n: 1, back: true}, {ch: q, n: 5}, {ch: b, n: 4}, {ch: a, n: 3},
	}
	for _, whole := range []bool{false, true} {
		c := newCounter(Options{Budget: counterBudget})
		c.whole = whole
		m := &mapCounter{whole: whole, per: map[string]int64{}, spent: map[charge]int64{}}
		for i, op := range ops {
			if got, want := applyTo(c, op), m.apply(op); got != want {
				t.Errorf("whole %t, op %d: pay %t, want %t", whole, i, got, want)
			}
			sameCounts(t, c, m, i, []charge{a, b, q})
		}
	}
}
