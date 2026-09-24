package eval_test

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
)

// STDLIB.md §2.2, §4.3: -0.0 orders before +0.0 in min, max, clamp and xs.min(), on the sign bit.
func TestSignedZero(t *testing.T) {
	for _, c := range []struct {
		expr     string
		negative bool
	}{
		{"min(0.0, -0.0)", true},
		{"min(-0.0, 0.0)", true},
		{"max(-0.0, 0.0)", false},
		{"max(0.0, -0.0)", false},
		{"clamp(-0.0, 0.0, 1.0)", false},
		{"clamp(0.0, -1.0, -0.0)", true},
		{"[0.0, -0.0].min()!", true},
		{"[-0.0, 0.0].max()!", false},
	} {
		t.Run(c.expr, func(t *testing.T) {
			b := buildFiles(t, eval.Options{}, "a/a.canon", pkgA+"local let x: Float = "+c.expr+"\n")
			f, ok := b.values[eval.Root{Pkg: "a", Name: "x"}].(*value.Float)
			if !ok || f.V != 0 || math.Signbit(f.V) != c.negative {
				t.Errorf("%s = %v, want a zero of sign bit %t\n%s", c.expr, f, c.negative, b.findings(t))
			}
		})
	}
}

// EVALUATION.md §2.2: `a[i] op= e` reads a[i] before it evaluates e: the missing element fails.
func TestCompoundReadsFirst(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "a/a.canon", pkgA+`local let zero: Int = 0

local fn f() -> Int {
  var ys = [1]
  ys[5] += 1 / zero
  return ys[0]
}

local let x: Int = f()
`)
	out := b.findings(t)
	if !strings.Contains(out, "["+codeOf(diag.E4002)+"]") || strings.Contains(out, "["+codeOf(diag.E4102)+"]") {
		t.Errorf("findings:\n%s", out)
	}
}

// EVALUATION.md §8.4: a failed template's message is its source, one delimiter off each end.
func TestTemplateText(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "a/a.canon", pkgA+`local let xs: [Int] = [1]

check xs.len() > 5 else "few {xs[9]} \""

check xs.len() > 6 else """
  far {xs[9]}
  """
`)
	var msgs []string
	for _, f := range b.bags["a"].Findings() {
		if f.Code == diag.E5001.Def().Code {
			msgs = append(msgs, f.Message)
		}
	}
	want := []string{`few {xs[9]} \"`, "\n  far {xs[9]}\n  "}
	if strings.Join(msgs, "|") != strings.Join(want, "|") {
		t.Errorf("messages %q, want %q\n%s", msgs, want, b.findings(t))
	}
}

// growNode is a record tree and a function that doubles its sharing n times.
const growNode = `/// A node.
record Node {
  /// Its children.
  kids: [Node]
}

local fn grow(n: Int) -> Node {
  var x = Node { kids: [] }
  var i = 0
  while i < n {
    x = Node { kids: [x, x] }
    i += 1
  }
  return x
}
`

// DECISIONS 199: a set hashes a bounded prefix of each element and lets the charged equality
// decide, so two trees shared 2^60 ways make one element within a small budget.
func TestUniqueOfTowers(t *testing.T) {
	b := buildFiles(t, eval.Options{Budget: towerBudget}, "a/a.canon", pkgA+growNode+"\nlocal let x: Int = [grow(60), grow(60)].unique().len()\n")
	if v, ok := b.values[eval.Root{Pkg: "a", Name: "x"}]; !ok || v.CanonText() != "1" {
		t.Errorf("x = %v\n%s", v, b.findings(t))
	}
}

// towerBudget is a small budget: the few thousand steps building the trees and comparing
// their distinct nodes take.
const towerBudget = 20_000

// EVALUATION.md §3.4, DECISIONS 199: binding rebuilds a shared node once; no stage B here.
func TestBindingOverTower(t *testing.T) {
	src := pkgA + growNode + `
/// Item.
record Item {
  /// Id.
  id: String
}

/// Shop.
record Shop {
  /// Items.
  items: [Item] keyed by id
  /// Fav.
  fav: ref Item
  /// Tree.
  tree: Node
}

/// S.
let s: Shop = { items: [{ id: "a" }], fav: "a", tree: grow(40) }

/// Fav.
let fav: String = s.fav.id
`
	if v, ok, fs := forceAlone(t, src, towerBudget, "fav"); !ok || v.CanonText() != "a" {
		t.Errorf("fav = %v, %t: %v", v, ok, fs)
	}
}

// forceAlone forces one let of package a with a host-less evaluator: no stage B, no checks.
func forceAlone(t *testing.T, src string, budget int64, name string) (value.Value, bool, []diag.Finding) {
	t.Helper()
	ctx := context.Background()
	p := parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(src)})
	bags := check.Bags{}
	opt := eval.Options{Budget: budget}
	ev := eval.New(check.Check(ctx, exampleProject(), p.files, bags, eval.NewFolder(bags, opt)), nil, bags, opt)
	v, ok := ev.Force(ctx, eval.Root{Pkg: "a", Name: name})
	return v, ok, bags["a"].Findings()
}

// doubleFn builds a list of 2^n zeros by doubling.
const doubleFn = `local fn double(n: Int) -> [Int] {
  var xs = [0]
  var i = 0
  while i < n {
    xs = xs + xs
    i += 1
  }
  return xs
}
`

// bigBudget covers building a list of 10^7 elements by doubling, a step per element made.
const bigBudget = 50_000_000

// DECISIONS 199: a set hashes the first nodes of each element and pushes only the components
// it reaches, so elements that hold a list of 10^7 cost no more than small ones.
func TestUniqueOverBigElements(t *testing.T) {
	src := pkgA + doubleFn + `
local let big: [Int] = double(23) + double(21)

local let x: Int = [[[i], big] for i in 0..100].unique().len()
`
	if v, ok, fs := forceAlone(t, src, bigBudget, "x"); !ok || v.CanonText() != "100" {
		t.Errorf("x = %v, %t: %v", v, ok, fs)
	}
}

// DECISIONS 199: a map hashes by its size and the sum of its entry hashes, so 10^4 distinct
// one-key maps fall in distinct hashes and unique costs a few steps per map.
func TestUniqueOfMaps(t *testing.T) {
	src := pkgA + "local let maps: [{Int: Int}] = [{ (i): 1 } for i in 0..10000]\n\nlocal let x: Int = maps.unique().len()\n"
	if v, ok, fs := forceAlone(t, src, mapsBudget, "x"); !ok || v.CanonText() != "10000" {
		t.Errorf("x = %v, %t: %v", v, ok, fs)
	}
}

// mapsBudget is linear in the 10^4 maps: building each and taking it into the set.
const mapsBudget = 200_000

// EVALUATION.md §3.4, DECISIONS 199: spreads over a record holding a large list skip the list.
func TestSpreadsOverSharedList(t *testing.T) {
	src := pkgA + doubleFn + `
/// Item.
record Item {
  /// Id.
  id: String
}

/// Shop.
record Shop {
  /// Items.
  items: [Item] keyed by id
  /// Fav.
  fav: ref Item
  /// Big.
  big: [Int]
  /// Name.
  name: Int
}

/// Base.
let base: Shop = { items: [{ id: "a" }], fav: "a", big: double(20), name: 0 }

/// Shops.
let shops: [Shop] = [{ ...base, name: n } for n in 0..2000]

/// N.
let n: Int = shops.len()
`
	if v, ok, fs := forceAlone(t, src, bigBudget, "n"); !ok || v.CanonText() != "2000" {
		t.Errorf("n = %v, %t: %v", v, ok, fs)
	}
}
