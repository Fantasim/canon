package eval_test

import (
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/eval"
)

// scaleLimit is how long a built-in over scaleN elements may take, the whole build included.
const scaleLimit = time.Second

// TYPES.md §7.5 equality at scale: each built-in over 20000 elements ends within scaleLimit.
func TestBuiltinsScale(t *testing.T) {
	head := "local let xs: [Int] = [i for i in 0..20000]\nlocal let ys: [Int] = [i % 100 for i in 0..20000]\n"
	for _, c := range []struct{ typ, expr, want string }{
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
	} {
		t.Run(c.expr, func(t *testing.T) {
			src := []byte("/// A.\npackage a\n\n" + head + "local let x: " + c.typ + " = " + c.expr + "\n")
			start := time.Now()
			b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{src}), eval.Options{})
			if d := time.Since(start); d > scaleLimit {
				t.Errorf("took %v", d)
			}
			if v, ok := b.values[eval.Root{Pkg: "a", Name: "x"}]; !ok || v.CanonText() != c.want {
				t.Errorf("got %v, want %s\n%s", v, c.want, b.findings(t))
			}
		})
	}
}
