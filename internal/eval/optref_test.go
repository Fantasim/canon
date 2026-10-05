package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// optRefHead declares a keyed list with a ref C? and a ref C into it, for equality between them.
const optRefHead = `/// A.
package a

local record C { name: String }

local record O {
  cols: [C] keyed by name
  pick: ref C?
  same: ref C
}

`

// TYPES.md §7.5 (TYP-08): equality between ref C and ref C? is symmetric, whichever side is optional.
func TestOptionalRefEqualitySymmetric(t *testing.T) {
	for _, c := range []struct{ name, pick, same, want string }{
		{"equal", "a", "a", "[true, true, false, false]"},
		{"unequal", "a", "b", "[false, false, true, true]"},
		{"none", "none", "a", "[false, false, true, true]"},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := optRefHead + `local let o: O = { cols: [{ name: "a" }, { name: "b" }], pick: ` + c.pick + `, same: ` + c.same + ` }
local let x: [Bool] = [o.same == o.pick, o.pick == o.same, o.same != o.pick, o.pick != o.same]
`
			b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(src)}), eval.Options{})
			v, ok := b.values[eval.Root{Pkg: "a", Name: "x"}]
			if !ok || v.CanonText() != c.want {
				t.Errorf("got %v %t, want %s\n%s", v, ok, c.want, b.findings(t))
			}
		})
	}
}

// TYPES.md §7.5 (TYP-08): inside a record check, same == pick and pick == same agree.
func TestOptionalRefEqualityInCheck(t *testing.T) {
	src := `/// A.
package a

local record C { name: String }

local record O {
  cols: [C] keyed by name
  pick: ref C?
  same: ref C
  check same == pick else "same != pick"
  check pick == same else "pick != same"
}

local let o: O = { cols: [{ name: "a" }, { name: "b" }], pick: a, same: a }
`
	b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(src)}), eval.Options{})
	if got := b.findings(t); got != noFindings {
		t.Errorf("findings:\n%s", got)
	}
}
