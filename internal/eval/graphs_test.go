package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// STDLIB.md §2.3, TYPES.md §6.2: a successor given as a ref or as an entry is the same node.
func TestGraphsOverEntries(t *testing.T) {
	head := `/// A.
package a

/// Step.
record Step {
  /// After.
  after: [ref Step] = []
}

/// Steps.
let steps: table Step = {
  build { after: [test] }
  test { after: [ship] }
  ship { after: [] }
}

/// The first.
let first: ref Step = build
`
	for _, c := range []struct{ decl, want string }{
		{"local let x: [ref Step] = reachable(from: first, next: .after)", "[build, test, ship]"},
		{"local let x: [ref Step] = reachable(from: steps.build, next: s => s.after)", "[build, test, ship]"},
		{"local let x: [Step] = reachable(from: steps.build, next: s => s.after)", "[Step{after: [test]}, Step{after: [ship]}, Step{after: []}]"},
		{"local let x: Int = cycles([steps.build], next: s => s.after).len()", "0"},
		{"local let x: [ref Step] = topoSort(steps.keys(), next: s => s.after)", "[ship, test, build]"},
		{"local let x: Int = topoSort(steps, next: s => s.after).len()", "3"},
	} {
		t.Run(c.decl, func(t *testing.T) {
			b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(head + c.decl + "\n")}), eval.Options{})
			v, ok := b.values[eval.Root{Pkg: "a", Name: "x"}]
			if !ok || v.CanonText() != c.want {
				t.Errorf("got %v %t, want %s\n%s", v, ok, c.want, b.findings(t))
			}
		})
	}
}
