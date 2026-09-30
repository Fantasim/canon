package eval_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
)

// variantChecks declares a variant's checks outside its cases, and a value of each case.
const variantChecks = `/// A.
package a

local variant Reward {
  item { count: Int }
  gold { amount: Int }
  nothing

  fn worth(self) -> Int {
    return match self {
      item(i) => i.count
      gold(g) => g.amount
      nothing => 0
    }
  }

  check positive: worth() >= 0 else "{self.kind} is worth {worth()}"

  check {
    if self is gold {
      if self.amount > 100 { fail(self, "too much gold") }
    }
  }
}

local let debt: Reward = gold { amount: -2 }
local let hoard: Reward = gold { amount: 500 }
local let loot: Reward = item { count: 3 }
local let empty: Reward = nothing
`

// TYPES.md §12.1 (DECISIONS 226), EVALUATION.md §8.3, §8.4: a variant-level check runs with `self` the variant value, whatever case it holds.
func TestVariantLevelChecks(t *testing.T) {
	b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(variantChecks)}), eval.Options{})
	positive, block := variantLevel(t, b)
	cases := []struct {
		value string
		check *syntax.CheckDecl
		want  string
	}{
		{"debt", positive, "failed: gold is worth -2"},
		{"hoard", positive, "holds"},
		{"loot", positive, "holds"},
		{"empty", positive, "holds"},
		{"hoard", block, "reports: gold{amount: 500}: too much gold"},
		{"debt", block, "holds"},
		{"loot", block, "holds"},
	}
	for _, c := range cases {
		v, ok := b.values[eval.Root{Pkg: "a", Name: c.value}]
		if !ok {
			t.Fatalf("%s: not evaluated", c.value)
		}
		if got := describeRun(b.ev.Run(context.Background(), c.check, v, c.value)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.value, got, c.want)
		}
	}
}

// variantLevel is the fixture's named one-line check and its block check, both outside any case.
func variantLevel(t *testing.T, b *build) (named, block *syntax.CheckDecl) {
	t.Helper()
	for _, d := range b.prog.files[0].Decls {
		v, ok := d.(*syntax.VariantDecl)
		if !ok {
			continue
		}
		for _, it := range v.Items {
			if c, isCheck := it.(*syntax.CheckDecl); isCheck && c.Body != nil {
				block = c
			} else if isCheck {
				named = c
			}
		}
	}
	if named == nil || block == nil {
		t.Fatal("the fixture's variant-level checks are missing")
	}
	return named, block
}

// describeRun is a check run as text: aborted, failed with its message, its reports, or holds.
func describeRun(r eval.CheckRun) string {
	switch {
	case r.Aborted:
		return "aborted"
	case r.Failed:
		return "failed: " + r.Message
	case len(r.Reports) == 0:
		return "holds"
	}
	parts := []string{"reports:"}
	for _, rep := range r.Reports {
		parts = append(parts, rep.At.CanonText()+": "+rep.Message)
	}
	return strings.Join(parts, " ")
}
