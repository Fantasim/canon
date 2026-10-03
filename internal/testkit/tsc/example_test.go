package tsc_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/tsc"
)

func ExampleArgs() {
	fmt.Println(tsc.Args("types", "a.ts")[0], len(tsc.Args("types", "a.ts", "b.ts")))
	// Output: --strict 14
}
