package grammar_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/progen"
	"github.com/fantasim/canonlang/internal/testkit/progen/grammar"
)

// A generated file parses without a finding; corrupted, it gets errors, all inside the file.
func Example() {
	r := progen.NewRand(1)
	src := grammar.Generate(r, progen.NewBudget(40, 4))
	tree, findings := grammar.Parse("gen/gen.canon", src)
	fmt.Println(len(findings), tree.Package != nil)
	bad := grammar.Corrupt(r, append(src, "let = )\n"...))
	_, findings = grammar.Parse("gen/gen.canon", bad)
	fmt.Println(len(findings) > 0)
	// Output:
	// 0 true
	// true
}
