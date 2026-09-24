package conform

import "slices"

// combine is the index tuples of lists of these sizes, in lexicographic order (CONFORMANCE.md §6.3).
func combine(sizes []int) [][]int {
	if slices.Contains(sizes, 0) {
		return nil
	}
	if len(sizes) == 1 {
		out := make([][]int, sizes[0])
		for i := range out {
			out[i] = []int{i}
		}
		return out
	}
	if product(sizes) <= maxProduct {
		return cartesian(sizes)
	}
	return pairwise(sizes)
}

// product is the number of tuples, saturated past maxProduct.
func product(sizes []int) int {
	n := 1
	for _, s := range sizes {
		n *= s
		if n > maxProduct {
			return n
		}
	}
	return n
}

// cartesian is every tuple, the first index varying slowest.
func cartesian(sizes []int) [][]int {
	var out [][]int
	t := make([]int, len(sizes))
	for {
		out = append(out, slices.Clone(t))
		k := len(t) - 1
		for k >= 0 && t[k] == sizes[k]-1 {
			t[k] = 0
			k--
		}
		if k < 0 {
			return out
		}
		t[k]++
	}
}

// pair is one value index of list i beside one of list j, i < j.
type pair struct {
	i, a, j, b int
}

// cover is the pairwise construction of CONFORMANCE.md §6.3, before sorting.
type cover struct {
	sizes   []int
	tuples  [][]int
	covered map[pair]bool
}

// pairwise covers every pair of values of every two lists, deterministically.
func pairwise(sizes []int) [][]int {
	c := &cover{sizes: sizes, covered: map[pair]bool{}}
	for i := range sizes {
		for j := i + 1; j < len(sizes); j++ {
			c.coverLists(i, j)
		}
	}
	slices.SortFunc(c.tuples, slices.Compare[[]int])
	return slices.CompactFunc(c.tuples, slices.Equal[[]int])
}

// coverLists adds a tuple for each pair of values of lists i and j not covered yet.
func (c *cover) coverLists(i, j int) {
	for a := range c.sizes[i] {
		for b := range c.sizes[j] {
			if !c.covered[pair{i, a, j, b}] {
				c.add(i, a, j, b)
			}
		}
	}
}

// add appends one tuple of the pairwise construction and covers its pairs (CONFORMANCE.md §6.3).
func (c *cover) add(i, a, j, b int) {
	t := make([]int, len(c.sizes))
	for k, n := range c.sizes {
		t[k] = len(c.tuples) % n
	}
	t[i], t[j] = a, b
	c.tuples = append(c.tuples, t)
	for k := range t {
		for l := k + 1; l < len(t); l++ {
			c.covered[pair{k, t[k], l, t[l]}] = true
		}
	}
}
