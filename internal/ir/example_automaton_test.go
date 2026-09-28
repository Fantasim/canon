package ir_test

import (
	"fmt"
	"regexp"

	"github.com/fantasim/canonlang/internal/ir"
)

// `^a+$` as states: `^`, a step over `a`, a split looping back to it, `$`, accept.
func ExampleCompilePattern() {
	a, err := ir.CompilePattern(regexp.MustCompile(`^a+$`))
	var ops []ir.PatternOp
	for _, s := range a.States {
		ops = append(ops, s.Op)
	}
	fmt.Println(ops, a.States[1].Runes, a.States[2].Out, a.States[2].Alt, err)
	// Output: [2 3 1 2 0] [97 97] 1 3 <nil>
}
