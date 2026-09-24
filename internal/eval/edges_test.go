package eval_test

import (
	"math"
	"strings"
	"testing"

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
