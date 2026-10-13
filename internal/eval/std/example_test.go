package std_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// STDLIB.md §9.5: exact binary rounding, ties away from zero; a zero-padded hexadecimal Int.
func Example() {
	f := func(v float64, s syntax.FormatSpec) string { return std.Format(&value.Float{V: v}, s) }
	hex := syntax.FormatSpec{Decimals: syntax.NoDecimals, Zero: true, Width: 8, Hex: 'x'}
	fmt.Println(f(0.125, syntax.FormatSpec{Decimals: 2}), f(2.675, syntax.FormatSpec{Decimals: 2}), f(-1234.5, syntax.FormatSpec{Comma: true, Decimals: 2}), std.Format(&value.Int{V: 255}, hex))
	k, ok := std.KeyOf(&value.Int{V: 3})
	fmt.Println(k.Text(), ok, std.Less(&value.Str{V: "a"}, &value.Str{V: "b"}))
	// Output:
	// 0.13 2.67 -1,234.50 000000ff
	// 3 true true
}
