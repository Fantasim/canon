package std_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/value"
)

// STDLIB.md §9.5: a format spec rounds a Float from its exact binary value, ties away from zero.
func Example() {
	f := func(v float64, s std.Spec) string { return std.Format(&value.Float{V: v}, s) }
	fmt.Println(f(0.125, std.Spec{Decimals: 2}), f(2.675, std.Spec{Decimals: 2}), f(-1234.5, std.Spec{Comma: true, Decimals: 2}))
	k, ok := std.KeyOf(&value.Int{V: 3})
	fmt.Println(k.Text(), ok, std.Less(&value.Str{V: "a"}, &value.Str{V: "b"}))
	// Output:
	// 0.13 2.67 -1,234.50
	// 3 true true
}
