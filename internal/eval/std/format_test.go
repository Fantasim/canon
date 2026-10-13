package std_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// specRow is one formatted value: its type, its literal, the spec and the text it gives.
type specRow struct {
	rule, typ, val, spec, want string
}

// specTable is every row of STDLIB.md §9.5's table, then the edges DECISIONS 358 names.
var specTable = []specRow{
	{"STDLIB.md §9.5 table", "Int", "1234567", ",", "1,234,567"},
	{"STDLIB.md §9.5 table", "Float", "-1234.5", ",.2", "-1,234.50"},
	{"STDLIB.md §9.5 table", "Float", "0.125", ".2", "0.13"},
	{"STDLIB.md §9.5 table", "Float", "2.675", ".2", "2.67"},
	{"STDLIB.md §9.5 table", "Int", "500", ".2", "500.00"},
	{"STDLIB.md §9.5 table", "Int", "5", "+", "+5"},
	{"STDLIB.md §9.5 table", "Int", "0", "+", "+0"},
	{"STDLIB.md §9.5 table", "Int", "-3", "+", "-3"},
	{"STDLIB.md §9.5 table", "Int", "255", "08x", "000000ff"},
	{"STDLIB.md §9.5 table", "Int", "87", "02X", "57"},
	{"STDLIB.md §9.5 table", "Int", "-255", "x", "-ff"},
	{"STDLIB.md §9.5 table", "Int", "-5", "04", "-005"},
	{"STDLIB.md §9.5 table", "Int", "42", "5", "   42"},
	{"STDLIB.md §9.5 table", "Int", "3", "+03", "+03"},
	{"STDLIB.md §9.5 table", "Int", "1234567", "12,", "   1,234,567"},
	{"DECISIONS 358 zero in hex", "Int", "0", "x", "0"},
	{"DECISIONS 358 uppercase hex", "Int", "48879", "X", "BEEF"},
	{"DECISIONS 358 hex is the magnitude, never two's complement", "Int", "-9223372036854775808", "x", "-8000000000000000"},
	{"DECISIONS 358 hex padded with spaces", "Int", "-255", "6x", "   -ff"},
	{"DECISIONS 358 zeros after the sign, the sign counted", "Int", "-255", "+06x", "-000ff"},
	{"DECISIONS 358 + on hex", "Int", "255", "+x", "+ff"},
	{"DECISIONS 358 a longer text is never cut", "Int", "123456", "3", "123456"},
	{"DECISIONS 358 width on a Float, zeros", "Float", "-1.5", "08.2", "-0001.50"},
	{"DECISIONS 358 all-zero digits take +, then zeros", "Float", "-0.001", "+08.2", "+0000.00"},
	{"DECISIONS 358 width on a canonical Float", "Float", "0.1", "5", "  0.1"},
	{"DECISIONS 358 width on an exponent form", "Float", "1e21", "8,", "   1e+21"},
	{"DECISIONS 358 width 99", "Int", "7", "099", strings.Repeat("0", 98) + "7"},
}

// specSource declares one value and one formatted string per row: v<i> and s<i>.
func specSource(rows []specRow) []byte {
	var b strings.Builder
	b.WriteString("/// A.\npackage a\n\n")
	for i, r := range rows {
		fmt.Fprintf(&b, "local let v%d: %s = %s\nlocal let s%d: String = \"{v%d:%s}\"\n", i, r.typ, r.val, i, i, r.spec)
	}
	return []byte(b.String())
}

// STDLIB.md §9.5, DECISIONS 358: each spec through the lexer, the checker and the evaluator.
func TestFormatSpecTable(t *testing.T) {
	a := &txtar.Archive{Files: []txtar.File{{Name: "a/a.canon", Data: specSource(specTable)}}}
	out, values := evaluateAll(t, a)
	if !strings.Contains(out, "0 errors") {
		t.Fatalf("findings:\n%s", out)
	}
	for i, r := range specTable {
		if got := values[fmt.Sprintf("a.s%d", i)]; got != r.want {
			t.Errorf("%s: %s %s with :%s = %q, want %q", r.rule, r.typ, r.val, r.spec, got, r.want)
		}
	}
}

// STDLIB.md §9.5, DECISIONS 358: Format on a spec built by hand; `x` reads an Int only.
func TestFormatDirect(t *testing.T) {
	hex := syntax.FormatSpec{Decimals: syntax.NoDecimals, Zero: true, Width: 4, Hex: syntax.HexUpper}
	if got := std.Format(&value.Int{V: 171}, hex); got != "00AB" {
		t.Errorf("171 with :04X = %q, want 00AB", got)
	}
	plain := syntax.FormatSpec{Decimals: syntax.NoDecimals}
	if got := std.Format(&value.Float{V: 0.5, T: types.FloatType}, plain); got != "0.5" {
		t.Errorf("0.5 with no part = %q, want its canonical form", got)
	}
}

// STDLIB.md §1.3: Prepare's Len, charged first, is the length of the text String then builds.
func TestPreparedLen(t *testing.T) {
	float := func(f float64) value.Value { return &value.Float{V: f, T: types.FloatType} }
	for _, c := range []struct {
		v value.Value
		s syntax.FormatSpec
	}{
		{&value.Int{V: 7}, syntax.FormatSpec{Decimals: syntax.NoDecimals, Zero: true, Width: 99}},
		{&value.Int{V: -255}, syntax.FormatSpec{Decimals: syntax.NoDecimals, Width: 6, Hex: syntax.HexUpper}},
		{&value.Int{V: 1234567}, syntax.FormatSpec{Decimals: 20, Comma: true, Plus: true}},
		{float(1e300), syntax.FormatSpec{Decimals: 20, Width: 99}},
		{float(-0.001), syntax.FormatSpec{Decimals: 2, Plus: true, Zero: true, Width: 8}},
		{float(1e21), syntax.FormatSpec{Decimals: syntax.NoDecimals, Comma: true, Width: 3}},
	} {
		f := std.Prepare(c.v, c.s)
		if text := f.String(); f.Len() != len(text) || text != std.Format(c.v, c.s) {
			t.Errorf("%s with %+v: Len %d, text %q", c.v.CanonText(), c.s, f.Len(), text)
		}
	}
}

// formatSteps is the smallest budget x = tmpl, after n, does not run out (EVALUATION.md §12).
func formatSteps(t *testing.T, n, tmpl string) int64 {
	t.Helper()
	src := "/// A.\npackage a\n\nlocal let n: Int = " + n + "\nlocal let x: String = \"" + tmpl + "\"\n"
	a := &txtar.Archive{Files: []txtar.File{{Name: "a/a.canon", Data: []byte(src)}}}
	exhausted := "[" + string(diag.E4401.Def().Code) + "]"
	lo, hi := int64(1), int64(256)
	for lo < hi {
		mid := (lo + hi) / 2
		if out, _ := evaluateWith(t, a, eval.Options{Budget: mid}); strings.Contains(out, exhausted) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// STDLIB.md §1.3, DECISIONS 197, 358: a spec costs a step per byte it produces, padding included.
func TestFormatSpecSteps(t *testing.T) {
	for _, c := range []struct {
		n, a, b string
		extra   int64
	}{
		{"255", "{n:08x}", "{n:x}", 6},
		{"42", "{n:5}", "{n}", 3},
		{"-5", "{n:04}", "{n}", 2},
		{"255", "{n:X}", "{n:x}", 0},
		{"4095", "{n:x}", "{n}", -1},
		{"7", "{n:099}", "{n}", 98},
		{"7", "{n:99}", "{n:099}", 0},
	} {
		if got := formatSteps(t, c.n, c.a) - formatSteps(t, c.n, c.b); got != c.extra {
			t.Errorf("STDLIB.md §1.3: %s costs %d steps more than %s for %s, want %d", c.a, got, c.b, c.n, c.extra)
		}
	}
}
