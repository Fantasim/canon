package progen

// Budget bounds one generated case, in nodes and in nesting depth: a generator asks it before
// each node and writes a leaf once it is spent, so no seed can blow the test's memory.
type Budget struct {
	left, depth, maxDepth int
}

// NewBudget allows size nodes nested at most depth deep.
func NewBudget(size, depth int) *Budget {
	return &Budget{left: size, maxDepth: depth}
}

// Spend takes one node; false once the budget is spent.
func (b *Budget) Spend() bool {
	if b.left <= 0 {
		return false
	}
	b.left--
	return true
}

// Enter takes one node one level deeper, or nothing and false past the size or the depth.
func (b *Budget) Enter() bool {
	if b.depth >= b.maxDepth || b.left <= 0 {
		return false
	}
	b.left--
	b.depth++
	return true
}

// Leave closes the level the last successful Enter opened.
func (b *Budget) Leave() {
	if b.depth > 0 {
		b.depth--
	}
}
