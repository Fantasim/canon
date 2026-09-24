package eval_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// buildFiles runs a build over named sources.
func buildFiles(t *testing.T, opt eval.Options, files ...string) *build {
	t.Helper()
	var names []string
	var data [][]byte
	for i := 0; i+1 < len(files); i += 2 {
		names, data = append(names, files[i]), append(data, []byte(files[i+1]))
	}
	return runBuild(t, parseFiles(t, names, data), opt)
}

const pkgA = "/// A.\npackage a\n\n"

// EVALUATION.md §3.4, §4.1: binding a level-1 ref copies it; the shared value stays unbound.
func TestBindingCopies(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "a/a.canon", pkgA+`/// Item.
record Item {
  /// Id.
  id: String
  /// W.
  w: Int
}

/// Sub.
record Sub {
  /// Fav.
  fav: ref Item
}

/// Shop.
record Shop {
  /// Items.
  items: [Item] keyed by id
  /// Sub.
  sub: Sub
}

/// Shared.
let sub: Sub = { fav: "a" }

/// S1.
let s1: Shop = { items: [{ id: "a", w: 1 }], sub: sub }

/// S2.
let s2: Shop = { items: [{ id: "a", w: 2 }], sub: sub }

/// W1.
let w1: Int = s1.sub.fav.w

/// W2.
let w2: Int = s2.sub.fav.w
`)
	for name, want := range map[string]string{"w1": "1", "w2": "2"} {
		if v, ok := b.values[eval.Root{Pkg: "a", Name: name}]; !ok || v.CanonText() != want {
			t.Errorf("%s = %v, want %s", name, v, want)
		}
	}
	if out := b.findings(t); !strings.Contains(out, "sub.fav: ref Item has no enclosing Shop") {
		t.Errorf("findings:\n%s", out)
	}
}

// EVALUATION.md §7.3, DECISIONS 79: a value rebuilt from an invalid one stays invalid.
func TestInvalidCarried(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "a/a.canon", pkgA+`local let m: Int = 300

local let k: String = "a"

/// R.
record R {
  /// X.
  x: Int8
  /// Y.
  y: Int = 0
  /// M.
  m: {String: Int8} = {}
  check pos: x < 0 else "not negative"
}

/// A.
let a: R = R { x: m }

/// B.
let b: R = R { ...a, y: 1 }

/// C.
let c: R = R { x: -1 + 2, m: { (k): 1, "a": 2 } }
`)
	if out := b.findings(t); strings.Contains(out, "["+codeOf(diag.E5001)+"]") {
		t.Errorf("a check ran above an invalid value:\n%s", out)
	}
}

// EVALUATION.md §12.2, DECISIONS 186: E4401 is never captured by an expect; the test stops.
func TestBudgetInExpect(t *testing.T) {
	b := buildFiles(t, eval.Options{Budget: 500}, "a/a.canon", pkgA+`local fn spin(n: Int) -> Int {
  var i = 0
  while i < n { i += 1 }
  return i
}

test "budget" {
  expect spin(100000) fails `+codeOf(diag.E4401)+`
}
`)
	out := runTests(t, b)
	if !strings.Contains(out, "stopped true") || !strings.Contains(b.findings(t), "["+codeOf(diag.E4401)+"]") {
		t.Errorf("tests:\n%s\n%s", out, b.findings(t))
	}
}

// EVALUATION.md §14, TYPES.md §3.1: a finding goes to its own package's bag, a fold's included.
func TestFindingsPackages(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "a/a.canon", pkgA+"import b\n\n/// R.\nrecord R {\n  /// X.\n  x: Int(0..b.X)\n}\n\n/// U.\nlet u: R = { x: 1 }\n",
		"b/b.canon", "/// B.\npackage b\n\nconst ZERO = 0\n\n/// X.\nconst X = 10 / ZERO\n")
	if f := b.bags["b"].Findings(); len(f) != 1 || f[0].Code != diag.E4102.Def().Code {
		t.Errorf("b's findings: %v", f)
	}
	b = buildFiles(t, eval.Options{}, "a/a.canon", pkgA+"import b\n\nconst A = b.B + 1\n",
		"b/b.canon", "/// B.\npackage b\n\nimport a\n\nconst B = a.A + 1\n")
	if out := b.findings(t); strings.Contains(out, "["+codeOf(diag.E4301)+"]") {
		t.Errorf("findings:\n%s", out)
	}
}

// DECISIONS 148, 188: a where re-run costs nothing but always ends: at a count the size of
// the budget it is E4401.
func TestWhereEnds(t *testing.T) {
	b := buildFiles(t, eval.Options{Budget: 1000}, "a/a.canon", pkgA+`local fn spin(n: Int) -> Bool {
  var i = n
  while i >= 0 { i += 1 }
  return true
}

/// R.
record R {
  /// X.
  x: Int where spin(it)
}
`)
	var rt *types.RecordType
	for _, obj := range b.checked.Packages[0].Decls {
		if r, ok := obj.Type().(*types.RecordType); ok {
			rt = r
		}
	}
	pred := rt.Fields[0].Type.(*types.Refined).Where
	if _, ok := b.ev.Where(context.Background(), pred, &value.Int{V: 1, T: types.IntType}); ok {
		t.Error("an endless predicate held")
	}
	if out := b.findings(t); !strings.Contains(out, "["+codeOf(diag.E4401)+"]") {
		t.Errorf("findings:\n%s", out)
	}
}

// EVALUATION.md §13: a field a spread copies in a function keeps its origin; one finding.
func TestSpreadInFunction(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "a/a.canon", pkgA+`local let long: String = "abcdef"

/// R.
record R {
  /// X.
  x: String(1..3)
  /// Y.
  y: Int = 0
}

local fn copy(r: R) -> R { return R { ...r, y: 1 } }

/// A.
let a: R = R { x: long }

/// B.
let b: R = copy(a)
`)
	if n := strings.Count(b.findings(t), "["+codeOf(diag.E3204)+"]"); n != 1 {
		t.Errorf("%d %s findings:\n%s", n, codeOf(diag.E3204), b.findings(t))
	}
}

// DECISIONS 79, 188: a record a keyed list takes is the same instance, identity added.
func TestKeyedListKeepsInstance(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "a/a.canon", pkgA+`/// R.
record R {
  /// K.
  k: String
}

/// A.
let a: R = { k: "x" }

/// Xs.
let xs: [R] keyed by k = [a]
`)
	a := b.values[eval.Root{Pkg: "a", Name: "a"}]
	xs, ok := b.values[eval.Root{Pkg: "a", Name: "xs"}].(*value.List)
	if !ok || len(xs.Elems) != 1 || xs.Elems[0] != a {
		t.Errorf("xs = %v, a = %p", xs, a)
	}
}

// STDLIB.md §10, EVALUATION.md §6.1, DECISIONS 188: range lengths and slice bounds at the edges.
func TestRangeEdges(t *testing.T) {
	runCases(t, []evalCase{
		{"STDLIB.md §10 len overflow", "Int", "(-big - 1..big).len()", codeOf(diag.E4101)},
		{"STDLIB.md §10 descending len", "Int", "(seven..two).len()", "0"},
		{"EVALUATION.md §6.1 slice to ..=MAX", "[Int]", "xs[0..=big]", codeOf(diag.E4101)},
		{"STDLIB.md §4.1 negative inclusive bound", "[Int]", "xs[-2..=-1]", codeOf(diag.E4002)},
		{"STDLIB.md §7 the error quotes the bound as written", "String", `"aé"[-1..]`, codeOf(diag.E4107)},
	})
}
