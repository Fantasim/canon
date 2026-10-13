package std_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// duplicatesSource declares the lists the duplicates cases read.
const duplicatesSource = `/// A.
package a

/// A point, Eq but not a Key.
record P {
  /// Its x.
  x: Int
  /// Its tags.
  tags: [String] = []
}

/// Pets.
let pets: table P = {
  rex { x: 1 }
  tom { x: 2 }
}

local let words: [String] = ["a", "b", "b", "a"]
local let distinct: [Int] = [3, 1, 2]
local let empty: [Int] = []
local let order: [String] = ["c", "b", "a", "b", "c", "a", "d"]
local let many: [Int] = [5, 5, 5, 5]
local let points: [P] = [{ x: 1 }, { x: 2, tags: ["t"] }, { x: 1 }, { x: 2 }, { x: 2, tags: ["t"] }]
local let lists: [[Int]] = [[1], [2], [1], [1]]
local let refs: [ref pets] = [rex, tom, rex]

local let d1: [String] = words.duplicates()
local let d2: [Int] = distinct.duplicates()
local let d3: [Int] = empty.duplicates()
local let d4: [String] = order.duplicates()
local let d5: [Int] = many.duplicates()
local let d6: [P] = points.duplicates()
local let d7: [[Int]] = lists.duplicates()
local let d8: [ref pets] = refs.duplicates()
local let d9: Bool = order.duplicates() == order.unique().filter(w => order.count(v => v == w) > 1)
`

// STDLIB.md §4.2, DECISIONS 359: each repeated element once, in unique's order, on any Eq type.
func TestDuplicates(t *testing.T) {
	out, values := evaluateAll(t, &txtar.Archive{Files: []txtar.File{{Name: "a/a.canon", Data: []byte(duplicatesSource)}}})
	if !strings.Contains(out, "0 errors") {
		t.Fatalf("findings:\n%s", out)
	}
	for _, c := range []struct{ rule, name, want string }{
		{"DECISIONS 359 [a, b, b, a] gives [a, b]", "d1", `["a", "b"]`},
		{"STDLIB.md §4.2 no repeat", "d2", `[]`},
		{"STDLIB.md §4.2 empty receiver", "d3", `[]`},
		{"DECISIONS 359 first-seen order, not the order of the repeats", "d4", `["c", "b", "a"]`},
		{"DECISIONS 359 each listed once", "d5", `[5]`},
		{"DECISIONS 359 records by Eq", "d6", `[P{x: 1, tags: []}, P{x: 2, tags: ["t"]}]`},
		{"DECISIONS 359 lists by Eq", "d7", `[[1]]`},
		{"DECISIONS 359 refs by key", "d8", `[rex]`},
		{"DECISIONS 359 unique without the singletons", "d9", `true`},
	} {
		if got := values["a."+c.name]; got != c.want {
			t.Errorf("%s: %s = %s, want %s", c.rule, c.name, got, c.want)
		}
	}
}

// STDLIB.md §1.3, §4.2, DECISIONS 359: n steps plus one per pair of values compared.
func TestDuplicatesCost(t *testing.T) {
	strs := func(xs ...string) *value.List {
		l := &value.List{T: wordsType}
		for _, x := range xs {
			l.Elems = append(l.Elems, &value.Str{V: x, T: types.StringType})
		}
		return l
	}
	for _, c := range []struct {
		recv  *value.List
		pairs int
		want  string
	}{
		{strs(), 0, `[]`},
		{strs("a", "b", "c"), 0, `[]`},
		{strs("a", "b", "b", "a"), 2, `["a", "b"]`},
		{strs("a", "a", "a"), 2, `["a"]`},
	} {
		h := &countingHost{}
		got, ok := std.Method(h, &std.Call{Name: "duplicates", Recv: c.recv, Result: wordsType})
		if !ok || got == nil {
			t.Fatalf("%s.duplicates() failed", c.recv.CanonText())
		}
		if got.CanonText() != c.want {
			t.Errorf("%s.duplicates() = %s, want %s", c.recv.CanonText(), got.CanonText(), c.want)
		}
		if want := len(c.recv.Elems) + c.pairs; h.steps != want {
			t.Errorf("%s.duplicates(): %d steps, want n + %d pairs = %d", c.recv.CanonText(), h.steps, c.pairs, want)
		}
	}
}

// wordsType is the [String] of the cost cases.
var wordsType = &types.ListType{Elem: types.StringType}
