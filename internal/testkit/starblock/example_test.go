package starblock_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/starblock"
)

// Example shows a case: the doc text and the block both generators write for it.
func Example() {
	c := starblock.Cases()[0]
	fmt.Printf("%q\n%q\n", c.Doc, c.Block)
	// Output:
	// "first\nsecond"
	// "/**\n * first\n * second\n */\n"
}
