package ir_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// partsLimit is the budget the table cases cut against.
const partsLimit = 10

// CODEGEN.md §2.11, DECISIONS 435: PartEnds cuts a body of units between whole units into parts whose braces hold at most limit lines, greedily, the function's own lines (whole) counted in the uncut body and each part's own (part) in the part; it returns the exclusive end index of each part, and nil when no cut is needed or none helps.
func TestPartEnds(t *testing.T) {
	cases := []struct {
		name        string
		lines       []int
		limit       int
		whole, part int
		want        []int
	}{
		{"nil", nil, partsLimit, 0, 0, nil},
		{"empty", []int{}, partsLimit, 0, 0, nil},
		{"an empty body is no cut however small the budget", nil, 1, 2, 0, nil},
		{"units without a line are no cut however long the whole", []int{0, 0}, partsLimit, 20, 1, nil},
		{"under the limit", []int{3, 3}, partsLimit, 0, 0, nil},
		{"exact fit is not cut", []int{4, 6}, partsLimit, 0, 0, nil},
		{"one over the limit is cut", []int{4, 7}, partsLimit, 0, 0, []int{1, 2}},
		{"a single unit over the limit is no cut", []int{25}, partsLimit, 0, 0, nil},
		{"a single unit the whole's lines push over is one part", []int{999}, ir.PartLines, 2, 1, []int{1}},
		{"a unit over the limit stands alone", []int{2, 25, 3}, partsLimit, 0, 0, []int{1, 2, 3}},
		{"two oversized units", []int{25, 30}, partsLimit, 0, 0, []int{1, 2}},
		{"an oversized unit first, then small ones", []int{25, 4, 6, 1}, partsLimit, 0, 0, []int{1, 3, 4}},
		{"many small units", []int{3, 3, 3, 3, 3, 3, 3}, partsLimit, 0, 0, []int{3, 6, 7}},
		{"parts fill to the limit", []int{5, 5, 5, 5, 5}, partsLimit, 0, 0, []int{2, 4, 5}},
		{"zero-line units join the part", []int{0, 10, 0, 10}, partsLimit, 0, 0, []int{3, 4}},
		{"no cut before a zero-line unit after an oversized one", []int{25, 0}, partsLimit, 0, 0, nil},
		{"no cut before zero-line units after an oversized one", []int{11, 0, 0}, partsLimit, 0, 0, nil},
		{"zero-line units follow the cut", []int{6, 6, 0, 0}, partsLimit, 0, 0, []int{1, 4}},
		{"limit of one is a unit per part", []int{1, 1, 1}, 1, 0, 0, []int{1, 2, 3}},
		{"a whole that fills the limit with its own lines is not cut", []int{4, 3, 2}, partsLimit, 1, 0, nil},
		{"a whole one line over is cut", []int{4, 3, 3}, partsLimit, 1, 1, []int{2, 3}},
		{"a part that would be one over with its own lines is cut earlier", []int{4, 3, 3, 1}, partsLimit, 0, 1, []int{2, 4}},
		{"a part filling the limit with its own lines is kept", []int{5, 4, 5}, partsLimit, 0, 1, []int{2, 3}},
		{"a unit that alone fills the part is not cut off", []int{9, 9}, partsLimit, 0, 1, []int{1, 2}},
		{"a unit over the part's room stands alone", []int{1, 10, 1}, partsLimit, 0, 1, []int{1, 2, 3}},
		{"one part helps when the whole's lines are the excess", []int{4, 3, 3}, partsLimit, 2, 0, []int{3}},
		{"the shared budget, a whole of exactly the limit", []int{500, 498}, ir.PartLines, 2, 1, nil},
		{"the shared budget, a whole one over is one part", []int{500, 499}, ir.PartLines, 2, 1, []int{2}},
		{"the shared budget, a part one over with its return is cut earlier", []int{500, 499, 1}, ir.PartLines, 1, 1, []int{2, 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ir.PartEnds(c.lines, c.limit, c.whole, c.part); !slices.Equal(got, c.want) {
				t.Errorf("PartEnds(%v, %d, %d, %d) = %v, want %v", c.lines, c.limit, c.whole, c.part, got, c.want)
			}
		})
	}
}

// CODEGEN.md §2.11 (the budget is 1,000 lines), DECISIONS 435: one constant shared by every generator.
func TestPartLines(t *testing.T) {
	const budget = 1000
	if ir.PartLines != budget {
		t.Errorf("PartLines = %d, want %d", ir.PartLines, budget)
	}
}

// CODEGEN.md §3.5, §2.11, DECISIONS 435: the generated names of the parts: Decode<N> (C++), <fn>_<N> (Go, the interior _ lowerCamel never writes).
func TestPartNames(t *testing.T) {
	cases := []struct {
		name, got, want string
	}{
		{"C++ decode part 0", ir.CppDecodePart(0), "Decode0"},
		{"C++ decode part 12", ir.CppDecodePart(12), "Decode12"},
		{"Go build part", ir.GoPart("buildItems", 3), "buildItems_3"},
		{"Go decode part", ir.GoPart("decodeItem", 0), "decodeItem_0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("got %q, want %q", c.got, c.want)
			}
		})
	}
}
