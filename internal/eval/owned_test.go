package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// ownedCases are element assignments around every way a var's collection is shared: each
// program's let n is what rebuilding the root on every assignment gives.
var ownedCases = []struct {
	name, typ, body, want string
}{
	{"a let holds the map before the next assignment", "[{Int: Int}]", `
  var m: {Int: Int} = {}
  m[1] = 1
  m[2] = 2
  let c = m
  m[3] = 3
  m[1] = 9
  return [c, m]`, "[{1: 1, 2: 2}, {1: 9, 2: 2, 3: 3}]"},
	{"a function argument keeps the map it was given", "[{Int: Int}]", `
  var m: {Int: Int} = {}
  m[1] = 1
  m[2] = 2
  let k = keep(m)
  m[1] = 7
  let b = bump(m)
  m[2] = 8
  return [k, b, m]`, "[{1: 1, 2: 2}, {1: 8, 2: 2}, {1: 7, 2: 8}]"},
	{"a lambda captures the map when created", "[Int]", `
  var m: {Int: Int} = {}
  m[1] = 1
  m[2] = 2
  let g: fn(Int) -> Int = k => m[k]
  m[1] = 5
  return [g(1), m[1]]`, "[1, 5]"},
	{"a for loop walks the list as it was", "Int", `
  var xs = [1, 2, 3]
  xs[0] = 0
  var t = 0
  for x in xs {
    xs[1] = 100
    t += x
  }
  return t * 1000 + xs[1]`, "5100"},
	{"a row read out of a grid keeps its elements", "[[Int]]", `
  var g: [[Int]] = [[1, 2], [3]]
  g[0][0] = 7
  let row = g[0]
  g[0][1] = 8
  g[1][0] = row[1]
  return [row, g[0], g[1]]`, "[[7, 2], [7, 8], [2]]"},
	{"the keys listed before an assignment stay", "[Int]", `
  var m: {Int: Int} = { 1: 1 }
  m[2] = 2
  let ks = m.keys()
  m[3] = 3
  m[1] += 10
  return ks + [m.len(), m[1]]`, "[1, 2, 3, 11]"},
	{"a keyed lookup after an assignment in place reads the new entry", "[Int]", `
  var ks: [Item] keyed by k = [{ k: "k{i}" } for i in 0..20]
  ks["k1"] = { k: "k1", n: 5 }
  let first = ks["k1"].n
  ks["k1"] = { k: "k1", n: 6 }
  return [first, ks["k1"].n, ks.get("k1")!.n]`, "[5, 6, 6]"},
	{"a map updated in place equals one built whole", "Bool", `
  var a: {Int: Int} = { 1: 1, 2: 2 }
  a[1] = 5
  a[1] = 1
  a[3] = 3
  let b: [{Int: Int}] = [{ 1: 1, 2: 2, 3: 3 }]
  return a == b[0] and b.contains(a)`, "true"},
	{"a table var updated in place keeps the entries read and the table it copied", "[Int]", `
  var t = cells
  t["c1"] = { n: 1 }
  let before = t
  let one = t["c1"].n
  t["c1"] = { n: 5 }
  t["c2"] = { n: 7 }
  let two = t["c2"].n
  t["c2"] = { n: 8 }
  return [one, before["c1"].n, before["c2"].n, t["c1"].n, t["c2"].n, two, cells["c2"].n]`, "[1, 1, 0, 5, 8, 7, 0]"},
	{"a list literal holds the map before the next assignment", "[{Int: Int}]", `
  var m: {Int: Int} = {}
  m[1] = 1
  m[2] = 2
  let ys = [m]
  m[1] = 9
  return [ys[0], m]`, "[{1: 1, 2: 2}, {1: 9, 2: 2}]"},
	{"a record literal holds the map before the next assignment", "[{Int: Int}]", `
  var m: {Int: Int} = {}
  m[1] = 1
  m[2] = 2
  let b: Box = { m: m }
  m[1] = 9
  return [b.m, m]`, "[{1: 1, 2: 2}, {1: 9, 2: 2}]"},
	{"an interpolation reads the map as it was", "[String]", `
  var m: {Int: Int} = {}
  m[1] = 1
  m[2] = 2
  let s = "{m}"
  m[1] = 9
  return [s, "{m}"]`, `["{1: 1, 2: 2}", "{1: 9, 2: 2}"]`},
	{"each recursive frame owns its own var", "[{Int: Int}]", `
  return [fill(2), grow({}, 2)]`, "[{0: 2, 1: 2, 2: 1, 3: 2}, {2: 2, 102: 3}]"},
}

// ownedPrelude declares what ownedCases call: records, a table, a function returning its
// argument, functions updating their own copies, recursively too.
const ownedPrelude = `/// A.
package a

/// Cell.
record Cell {
  /// N.
  n: Int = 0
}

/// Cells, enough to be looked up through an index.
let cells: table Cell = {
  c0 {}
  c1 {}
  c2 {}
  c3 {}
  c4 {}
  c5 {}
  c6 {}
  c7 {}
  c8 {}
  c9 {}
  c10 {}
  c11 {}
  c12 {}
  c13 {}
  c14 {}
  c15 {}
  c16 {}
}

/// Box.
record Box {
  /// M.
  m: {Int: Int}
}

/// n at 0, 1 and 3 of a map built in its own frame, fill(n - 1)'s 0 at 2.
fn fill(n: Int) -> {Int: Int} {
  var m: {Int: Int} = {}
  m[0] = n
  m[1] = n
  if n > 0 {
    let inner = fill(n - 1)
    m[2] = inner[0]
  }
  m[3] = n
  return m
}

/// m with n at n, then 100 + n at the size of what the call below returned.
fn grow(m: {Int: Int}, n: Int) -> {Int: Int} {
  var c = m
  c[n] = n
  if n > 0 {
    let d = grow(c, n - 1)
    c[100 + n] = d.len()
  }
  return c
}

/// Item.
record Item {
  /// K.
  k: String
  /// N.
  n: Int = 0
}

/// Returns m.
fn keep(m: {Int: Int}) -> {Int: Int} {
  return m
}

/// Adds one to the value at 1 of its own copy of m.
fn bump(m: {Int: Int}) -> {Int: Int} {
  var c = m
  c[1] += 1
  return c
}
`

// EVALUATION.md §4.1 (EVL-04): a collection updated in place behaves as a rebuilt root.
func TestOwnedValueSemantics(t *testing.T) {
	for _, c := range ownedCases {
		src := ownedPrelude + "\n/// F.\nfn f() -> " + c.typ + " {" + c.body + "\n}\n\n/// N.\nlet n: " + c.typ + " = f()\n"
		b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(src)}), eval.Options{})
		got := "poisoned"
		if v, ok := b.values[eval.Root{Pkg: "a", Name: "n"}]; ok {
			got = v.CanonText()
		}
		if got != c.want {
			t.Errorf("%s: got %s, want %s\n%s", c.name, got, c.want, b.findings(t))
		}
	}
}
