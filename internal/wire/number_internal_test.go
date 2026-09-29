package wire

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// FORMATTER.md §14.1, WIRE.md §5.1, §7.2: each numeric reading's canonical text (log-2026-09-29 M4 U2b-r).
func TestNumberText(t *testing.T) {
	tests := []struct {
		name string
		v    value.Value
		tok  string
		want string
		ok   bool
	}{
		{"float shortest", &value.Float{V: 0.1, T: types.FloatType}, "0.10000000000000000555", "0.1", true},
		{"float integral", &value.Float{V: 100, T: types.FloatType}, "1e2", "100", true},
		{"float negative zero", &value.Float{V: 0, T: types.FloatType}, "-0.0", "0", true},
		{"float32 shortest", &value.Float{V: float64(float32(0.1)), T: types.Float32Type}, "0.10000000149011612", "0.1", true},
		{"float big", &value.Float{V: 1e21, T: types.FloatType}, "1000000000000000000000", "1e+21", true},
		{"float not this token", &value.Float{V: 2, T: types.FloatType}, "1.0", "", false},
		{"int negative zero", &value.Int{V: 0, T: types.IntType}, "-0", "0", true},
		{"int max", &value.Int{V: 1<<63 - 1, T: types.IntType}, "9223372036854775807", "9223372036854775807", true},
		{"bool as int", &value.Bool{V: true}, "1", "1", true},
		{"duration ms", &value.Dur{Ms: 8000}, "8e3", "8000", true},
		{"duration s", &value.Dur{Ms: 15000}, "1.50e1", "15", true},
		{"duration tiny digits", &value.Dur{Ms: 100}, "0.00000000000000000001e22", "100", true},
		{"duration zero", &value.Dur{Ms: 0}, "-0.0e9", "0", true},
		{"duration not whole", &value.Dur{Ms: 90000}, "1.5", "1.5", true},
		{"duration not this token", &value.Dur{Ms: 7}, "2", "", false},
		{"record", &value.Record{}, "1.50", "", false},
		{"none marker", &value.None{T: types.IntType}, "-1.0", "", false},
	}
	for _, tt := range tests {
		got, ok := NumberText(tt.v, tt.tok)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%s: NumberText(%q) = %q, %v; want %q, %v", tt.name, tt.tok, got, ok, tt.want, tt.ok)
		}
	}
}

// WIRE.md §3.3, §7.2: an integer reading's text is its token's exact value, refused only when inexact.
func FuzzIntegerText(f *testing.F) {
	for _, s := range []string{"0", "-0", "8e3", "1.50e1", "1.5", "125e-2", "9223372036854775807", "-0.000e+2", "0.00000000000000000001e22"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, tok string) {
		_, exp, hasExp := strings.Cut(strings.ToLower(tok), "e")
		if !json.Valid([]byte(tok)) || tok != strings.TrimSpace(tok) || !strings.ContainsAny(tok[:1], "-0123456789") || hasExp && len(strings.TrimLeft(exp, "+-")) > 3 {
			return
		}
		want, _ := new(big.Rat).SetString(tok)
		got, ok := integerText(parseDecimal(tok))
		limit := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(19), nil))
		if !ok {
			if want.IsInt() && new(big.Rat).Abs(want).Cmp(limit) < 0 {
				t.Fatalf("integerText(%q) refused an integer", tok)
			}
			return
		}
		if got != want.Num().String() || !want.IsInt() {
			t.Fatalf("integerText(%q) = %q, want %s", tok, got, want.RatString())
		}
	})
}
