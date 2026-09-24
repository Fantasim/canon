package load

import (
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
