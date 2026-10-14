package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// closureHead declares what the closure cases read.
const closureHead = `local record R {
  n: Int
}

local let rs: [R] = [R { n: 1 }, R { n: 2 }, R { n: 3 }]

local fn apply(f: fn(Int) -> Int, v: Int) -> Int {
  return f(v)
}

local fn adder(n: Int, v: Int) -> Int {
  return apply(x => x + n, v)
}

local fn escape() -> Int {
  var f: fn(Int) -> Int = x => x
  for i in [1, 2, 3] {
    let k = i * 10
    f = x => x + k
  }
  return f(1)
}

local fn snapshot() -> Int {
  var c = 1
  let f: fn(Int) -> Int = x => x + c
  c = 5
  return f(1) + c
}

local fn shadow(k: Int) -> [Int] {
  return [1, 2].map(k => k * 10)
}

local fn pairs(n: Int) -> [Int] {
  return [1, 2, 3].enumerate().map((i, v) => i * v + n)
}

local fn tri(n: Int) -> Int {
  if n == 0 {
    return 0
  }
  return [1].map(k => tri(n - k) + n)[0]
}
`

// closureCases are lambda calls over captures, each with the value it must give.
var closureCases = []struct{ name, typ, expr, want string }{
	{"lambda param hides a fn param of the same name", "[Int]", "shadow(7)", "[10, 20]"},
	{"nested lambdas capture outer params", "[[Int]]", "[1, 2].map(a => [10, 20].map(b => a + b))", "[[11, 21], [12, 22]]"},
	{"nested capture three deep", "[Int]", "[1].map(a => [2].map(b => [3].map(c => a * 100 + b * 10 + c)[0])[0])", "[123]"},
	{"shorthand inside a lambda", "[Int]", "[10, 20].map(k => rs.map(.n).sum() + k)", "[16, 26]"},
	{"lambda passed down keeps its captures", "[Int]", "[adder(1, 10), adder(2, 10)]", "[11, 12]"},
	{"lambda escapes its scope", "Int", "escape()", "31"},
	{"capture is a snapshot at creation", "Int", "snapshot()", "7"},
	{"recursion through a fn inside a lambda", "Int", "tri(4)", "10"},
	{"comprehension inside a lambda body", "[[Int]]", "[1, 2].map(a => [a + i for i in 0..2])", "[[1, 2], [2, 3]]"},
	{"two-parameter lambda destructures a pair beside a capture", "[Int]", "pairs(1)", "[1, 3, 7]"},
	{"capturing predicate", "Int", "rs.filter(r => r.n > rs[0].n).len()", "2"},
}

// closureSource is one closure case as a package.
func closureSource(typ, expr string) [][]byte {
	return [][]byte{[]byte("/// A.\npackage a\n\n" + closureHead + "\nlocal let x: " + typ + " = " + expr + "\n")}
}

// EVALUATION.md §4.1, §12.1, DECISIONS 185, TYPES.md §12.4: each closure case gives its value.
func TestClosureCaptures(t *testing.T) {
	for _, c := range closureCases {
		t.Run(c.name, func(t *testing.T) {
			b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, closureSource(c.typ, c.expr)), eval.Options{})
			if v, ok := b.values[eval.Root{Pkg: "a", Name: "x"}]; !ok || v.CanonText() != c.want {
				t.Errorf("got %v, want %s\n%s", v, c.want, b.findings(t))
			}
		})
	}
}

// predicateHead is a 10000-member list and fns whose predicates capture a parameter.
const predicateHead = `local let xs: [Int] = [i for i in 0..10000]

local fn firstFrom(limit: Int) -> Int? {
  return xs.first(m => m >= limit)
}

local fn allBelow(limit: Int) -> Bool {
  return xs.all(m => m <= limit)
}
`

// predicateCases call a capturing predicate once per member.
var predicateCases = []struct{ typ, expr string }{
	{"Int?", "firstFrom(9999)"},
	{"Bool", "allBelow(9999)"},
}

// BenchmarkCapturingPredicate times a capturing predicate called 10000 times, the build included.
func BenchmarkCapturingPredicate(b *testing.B) {
	for _, c := range predicateCases {
		b.Run(c.expr, func(b *testing.B) {
			src := [][]byte{[]byte("/// A.\npackage a\n\n" + predicateHead + "local let x: " + c.typ + " = " + c.expr + "\n")}
			b.ReportAllocs()
			for b.Loop() {
				runBuild(b, parseFiles(b, []string{"a/a.canon"}, src), eval.Options{})
			}
		})
	}
}
