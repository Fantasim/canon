package load

import (
	"math"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

func str(parts ...syntax.StringPart) *syntax.StringLit { return &syntax.StringLit{Parts: parts} }

// load reads its path argument and a field's default straight off the syntax tree (SPEC.md §5.4).
func TestPlainString(t *testing.T) {
	if s, ok := plainString(str(syntax.StringPart{Text: "data/*.json"})); !ok || s != "data/*.json" {
		t.Errorf("plain literal = %q, %v", s, ok)
	}
	if _, ok := plainString(str(syntax.StringPart{Text: "a"}, syntax.StringPart{Interp: &syntax.Interp{}})); ok {
		t.Error("an interpolation is not a plain string")
	}
}

// DECISIONS 173, meta/decisions/log-2026-09-24.md "load.dir review (M2)": a default supported
// accepts is a plain literal of the field's own base kind; a mismatch needs the evaluator.
func TestDefaultLiteralOK(t *testing.T) {
	cases := []struct {
		name string
		expr syntax.Expr
		kind types.Kind
		want bool
	}{
		{"none", &syntax.NoneLit{}, types.Int, true},
		{"bool", &syntax.BoolLit{}, types.Bool, true},
		{"int", &syntax.IntLit{}, types.Int, true},
		{"int on a float field", &syntax.IntLit{}, types.Float, false},
		{"duration", &syntax.DurationLit{}, types.Duration, true},
		{"plain string", str(syntax.StringPart{Text: "x"}), types.String, true},
		{"string on an int field", str(syntax.StringPart{Text: "x"}), types.Int, false},
		{"interpolated string", str(syntax.StringPart{Interp: &syntax.Interp{}}), types.String, false},
		{"identifier", &syntax.IdentExpr{Name: "other"}, types.Int, false},
	}
	for _, c := range cases {
		if got := defaultLiteralOK(c.expr, c.kind); got != c.want {
			t.Errorf("%s: defaultLiteralOK = %v, want %v", c.name, got, c.want)
		}
	}
}

// WIRE.md §6.8: unary "-", then add, shift, and, or, each left-associative; integers; skip causes.
func TestEvalDefineExpr(t *testing.T) {
	accepted := map[string]int64{"A": 16}
	cases := []struct {
		text string
		want int64
		ok   bool
	}{
		{"-1 << 2", -4, true}, {"1 << -2 + 3", 2, true}, {"1 + 2 << 1", 6, true}, {"1 | 2 & 3", 3, true},
		{"1 - 2 - 3", -4, true}, {"8 >> 1 >> 1", 2, true}, {"-8 >> 1", -4, true}, {"- -1", 1, true},
		{"--1", 1, true}, {"A - -A", 32, true}, {"-(1 << 63)", math.MinInt64, true},
		{"(1 << 63 << 37) >> 63 >> 27", 1024, true}, {"1uLL", 1, true}, {"0", 0, true}, {"0x1F", 31, true},
		{"017", 15, true}, {"((( 1 )))", 1, true}, {"(1 << 4) | 0x2", 18, true}, {"\t7\t", 7, true},
		{"1 << 63", 0, false}, {"1 << 64", 0, false}, {"1 >> -1", 0, false}, {"(1", 0, false},
		{"1)", 0, false}, {"()", 0, false}, {"1 2", 0, false}, {"08", 0, false}, {"0x", 0, false},
		{"B", 0, false}, {"1 +", 0, false}, {"<< 1", 0, false}, {"-", 0, false}, {"", 0, false},
		{"1 * 2", 0, false}, {"'A'", 0, false}, {"1 < < 2", 0, false}, {"(1))", 0, false},
	}
	for _, c := range cases {
		got, ok := evalDefineExpr(c.text, accepted)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("%q: %d, %v; want %d, %v", c.text, got, ok, c.want, c.ok)
		}
	}
}

// DECISIONS 195: no nesting depth reaches the host stack or is refused; 3M-deep input evaluates.
func TestEvalDefineExprDeep(t *testing.T) {
	const depth = 3_000_000
	cases := []struct {
		text string
		want int64
	}{
		{strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth), 1},
		{strings.Repeat("-", depth) + "1", 1},
		{strings.Repeat("- ", depth+1) + "1", -1},
		{strings.Repeat("-(", depth) + "2" + strings.Repeat(")", depth), 2},
		{strings.Repeat("(1 + ", depth/10) + "1" + strings.Repeat(")", depth/10), depth/10 + 1},
	}
	for _, c := range cases {
		if got, ok := evalDefineExpr(c.text, nil); !ok || got != c.want {
			t.Errorf("%.12q…: %d, %v; want %d", c.text, got, ok, c.want)
		}
	}
}
