package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
)

// scaleHead declares the lists every scale case reads.
const scaleHead = "local let xs: [Int] = [i for i in 0..20000]\nlocal let ys: [Int] = [i % 100 for i in 0..20000]\n"

// scaleCases are the built-ins that compare elements, over 20000 of them, with the value
// each must give.
var scaleCases = []struct{ typ, expr, want string }{
	{"Int", "(xs + xs).unique().len()", "20000"},
	{"Bool", "(xs + [0]).isUnique()", "false"},
	{"Int", "xs.intersect(ys).len()", "100"},
	{"Int", "xs.diff(ys).len()", "19900"},
	{"Int", "ys.union(xs).len()", "39900"},
	{"Int", "xs.groupBy(x => x % 100).len()", "100"},
	{"Int", "xs.toMap(x => x, x => x).len()", "20000"},
	{"Int", "reachable(from: 0, next: n => if n < 19999 { [n + 1] } else { [] }).len()", "20000"},
	{"Int", "cycles(xs, next: n => [(n + 1) % 20000]).len()", "20000"},
	{"Int", "topoSort(xs, next: n => if n > 0 { [n - 1] } else { [] }).len()", "20000"},
}

// scaleSource is one scale case as a package.
func scaleSource(typ, expr string) [][]byte {
	return [][]byte{[]byte("/// A.\npackage a\n\n" + scaleHead + "local let x: " + typ + " = " + expr + "\n")}
}

// TYPES.md §7.5, STDLIB.md §2.3, §4.2, DECISIONS 195: each scale case gives the spec's value.
func TestBuiltinsScale(t *testing.T) {
	for _, c := range scaleCases {
		t.Run(c.expr, func(t *testing.T) {
			b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, scaleSource(c.typ, c.expr)), eval.Options{})
			if v, ok := b.values[eval.Root{Pkg: "a", Name: "x"}]; !ok || v.CanonText() != c.want {
				t.Errorf("got %v, want %s\n%s", v, c.want, b.findings(t))
			}
		})
	}
}

// BenchmarkBuiltinsScale times each scale case, the whole build included.
func BenchmarkBuiltinsScale(b *testing.B) {
	for _, c := range scaleCases {
		b.Run(c.expr, func(b *testing.B) {
			src := scaleSource(c.typ, c.expr)
			for b.Loop() {
				runBuild(b, parseFiles(b, []string{"a/a.canon"}, src), eval.Options{})
			}
		})
	}
}
