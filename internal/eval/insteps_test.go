package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// inStepsPrelude declares what inStepsCases read: a keyed record and a table of it.
const inStepsPrelude = `/// A.
package a

/// Cell.
record Cell {
  /// K.
  k: String
  /// N.
  n: Int = 0
}

/// Cells.
let cells: table Cell = {
  c0 { k: "c0" }
  c1 { k: "c1" }
  c2 { k: "c2" }
  c3 { k: "c3" }
}
`

// inStepsCases are `in` tests of a var's collection before an assignment to it, with the steps
// a build of each spends, those of the evaluator before `in` read its collection as a peek.
var inStepsCases = []struct {
	name, body string
	steps      int64
}{
	{"map", `
  var m: {Int: Int} = {}
  for i in 0..20 {
    if not (i % 3 in m) { m[i % 3] = i }
  }
  return m.len()`, 219},
	{"list", `
  var xs = [0, 1, 2]
  for i in 0..10 {
    if i in xs { xs[0] = i + 5 }
  }
  return xs[0]`, 124},
	{"keyed list", `
  var ks: [Cell] keyed by k = [{ k: "k{i}" } for i in 0..8]
  for i in 0..8 {
    let e = ks["k{i}"]
    if e in ks { ks["k{i}"] = { k: "k{i}", n: i } }
  }
  return ks["k3"].n`, 338},
	{"table", `
  var t = cells
  for c in cells {
    if c in t { t[c.k] = { k: c.k, n: 1 } }
  }
  return t["c2"].n`, 83},
	{"first-wins loop", `
  var m: {String: Int} = {}
  for t in [{ "a": 1, "b": 2 }, { "b": 3, "c": 4 }, { "c": 5, "d": 6 }] {
    for k, v in t {
      if not (k in m) { m[k] = v }
    }
  }
  return m.len() * 100 + m["b"] + m["c"]`, 111},
}

// EVALUATION.md §12.1: `in` costs its node and a step per pair, its collection updated in place or not.
func TestInSteps(t *testing.T) {
	for _, c := range inStepsCases {
		src := inStepsPrelude + "\n/// F.\nfn f() -> Int {" + c.body + "\n}\n\n/// N.\nlet n: Int = f()\n"
		b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(src)}), eval.Options{})
		v, ok := b.values[eval.Root{Pkg: "a", Name: "n"}]
		if !ok {
			t.Errorf("%s: n is poisoned\n%s", c.name, b.findings(t))
			continue
		}
		if got := b.ev.StepsSpent(); got != c.steps {
			t.Errorf("%s: %d steps (n = %s), want %d", c.name, got, v.CanonText(), c.steps)
		}
	}
}
