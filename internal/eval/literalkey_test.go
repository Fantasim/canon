package eval_test

import (
	"math/big"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// LiteralKey is the one reading of a literal key, which the language server shares.
func TestLiteralKey(t *testing.T) {
	// TYPES.md §4.1
	huge := new(big.Int).Lsh(big.NewInt(1), 64)
	cases := []struct {
		name string
		e    syntax.Expr
		want value.Key
		ok   bool
	}{
		{"int", &syntax.IntLit{Value: big.NewInt(7)}, value.Key{I: 7, IsInt: true}, true},
		{"int beyond int64", &syntax.IntLit{Value: huge}, value.Key{IsInt: true}, false},
		{"raw string", &syntax.RawStringLit{Value: "a b"}, value.Key{S: "a b"}, true},
		{"string", &syntax.StringLit{Parts: []syntax.StringPart{{Text: "sword"}}}, value.Key{S: "sword"}, true},
		{"empty string", &syntax.StringLit{}, value.Key{}, true},
		{"interpolated", &syntax.StringLit{Parts: []syntax.StringPart{{Text: "a"}, {Interp: &syntax.Interp{}}}}, value.Key{}, false},
		{"name", &syntax.IdentExpr{Name: "sword"}, value.Key{}, false},
	}
	for _, c := range cases {
		got, ok := eval.LiteralKey(c.e)
		if ok != c.ok || ok && got != c.want {
			t.Errorf("%s: got %+v %v, want %+v %v", c.name, got, ok, c.want, c.ok)
		}
	}
}
