package eval_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// costCase is a program whose let n does work proportional to a size, by name; lib, when set,
// is the source of each package of costLibs the program imports.
type costCase struct {
	name   string
	source func(n int) string
	lib    func(pkg string, n int) string
}

// costLibs are the packages a case with a lib imports.
var costLibs = []string{"b", "c"}

// costCases are element assignment loops, one merging imported tables, and field reads through
// a ref, at a size.
var costCases = []costCase{
	{"mapAppend", func(n int) string {
		return fmt.Sprintf(`/// A.
package a

/// Counts the repeated values of xs, a map built one key at a time.
fn dupes(xs: [Int]) -> Int {
  var seen: {Int: Int} = {}
  var d = 0
  for x in xs {
    if seen.get(x) != none { d += 1 }
    seen[x] = x
  }
  return d
}

/// N.
let n: Int = dupes([i for i in 0..%d])
`, n)
	}, nil},
	{"mapReplace", func(n int) string {
		return fmt.Sprintf(`/// A.
package a

/// Adds one to every value of a map, one key at a time.
fn bump(m: {Int: Int}) -> Int {
  var c = m
  for k in m.keys() {
    c[k] = c[k] + 1
  }
  return c[0]
}

/// N.
let n: Int = bump({ i: i for i in 0..%d })
`, n)
	}, nil},
	{"listSet", func(n int) string {
		return fmt.Sprintf(`/// A.
package a

/// Adds one to every element of a list, one position at a time.
fn bump(xs: [Int]) -> Int {
  var ys = xs
  for i in 0..xs.len() {
    ys[i] = ys[i] + 1
  }
  return ys[0]
}

/// N.
let n: Int = bump([i for i in 0..%d])
`, n)
	}, nil},
	{"refRead", func(n int) string {
		var src strings.Builder
		src.WriteString("/// A.\npackage a\n\n/// Row.\nrecord Row {\n")
		reads := make([]string, 0, refFields)
		for i := range refFields {
			fmt.Fprintf(&src, "  /// F.\n  f%d: Int = %d\n", i, i)
			reads = append(reads, fmt.Sprintf("x.f%d", i))
		}
		src.WriteString("}\n\n/// Link.\nrecord Link {\n  /// R.\n  r: ref rows\n}\n\n/// Rows.\nlet rows: table Row = {\n")
		for i := range n {
			fmt.Fprintf(&src, "  r%d {}\n", i)
		}
		src.WriteString("}\n\n/// Links.\nlet links: [Link] = [\n")
		for i := range n {
			fmt.Fprintf(&src, "  { r: r%d },\n", i)
		}
		fmt.Fprintf(&src, `]

/// Sums the fields of every row, read through each link's ref.
fn sum() -> Int {
  var t = 0
  for l in links {
    let x = l.r
    t += %s
  }
  return t
}

/// N.
let n: Int = sum()
`, strings.Join(reads, " + "))
		return src.String()
	}, nil},
	{"mergeImported", func(int) string {
		return `/// A.
package a

import b
import c

/// The rows of every imported table, the first of each key kept.
fn merge() -> {String: String} {
  var m: {String: String} = {}
  for t in [b.names, b.more, c.names, c.more] {
    for k, v in t {
      if not (k in m) { m[k] = v }
    }
  }
  return m
}

/// N.
let n: Int = merge().len()
`
	}, func(pkg string, n int) string {
		return fmt.Sprintf(`/// L.
package %[1]s

/// Names, the same keys in every package.
let names: {String: String} = { "k{i}": "%[1]s{i}" for i in 0..%[2]d }

/// More names, keys of this package only.
let more: {String: String} = { "%[1]s{i}": "%[1]s{i}" for i in 0..%[2]d }
`, pkg, n)
	}},
}

// refFields is the number of fields refRead reads through each ref; benchSize, the smaller size
// BenchmarkCostScale times.
const (
	refFields = 20
	benchSize = 2_000
)

// costBuild builds case c at size n.
func costBuild(t testing.TB, c costCase, n int) *build {
	t.Helper()
	names, files := []string{"a/a.canon"}, [][]byte{[]byte(c.source(n))}
	if c.lib != nil {
		for _, pkg := range costLibs {
			names, files = append(names, pkg+"/"+pkg+".canon"), append(files, []byte(c.lib(pkg, n)))
		}
	}
	b := runBuild(t, parseFiles(t, names, files), eval.Options{})
	if _, ok := b.values[eval.Root{Pkg: "a", Name: "n"}]; !ok {
		t.Fatalf("%s at %d: n is poisoned\n%s", c.name, n, b.findings(t))
	}
	return b
}

// The sizes TestCostScale compares, and the growth it allows in the bytes allocated per element
// between them: a cost per operation that does not grow with the collection keeps it near 1.
const (
	costBase   = 500
	costScale  = 4
	costGrowth = 2
)

// EVALUATION.md §4.1, DECISIONS 199: element assignments, `in` tests and ref reads allocate linearly.
func TestCostScale(t *testing.T) {
	for _, c := range costCases {
		small := allocated(t, c, costBase)
		large := allocated(t, c, costBase*costScale)
		if large > costGrowth*costScale*small {
			t.Errorf("%s: %d bytes at %d elements, %d at %d: more than %dx per element", c.name, small, costBase, large, costBase*costScale, costGrowth)
		}
	}
}

// allocated is the bytes a build of case c at size n allocates.
func allocated(t *testing.T, c costCase, n int) uint64 {
	t.Helper()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	costBuild(t, c, n)
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// BenchmarkCostScale times each cost case at a size.
func BenchmarkCostScale(b *testing.B) {
	for _, c := range costCases {
		for _, n := range []int{benchSize, benchSize * costScale} {
			b.Run(fmt.Sprintf("%s/%d", c.name, n), func(b *testing.B) {
				for b.Loop() {
					costBuild(b, c, n)
				}
			})
		}
	}
}
