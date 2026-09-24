package cxx_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// Example shows the first flag every compile uses; it never invokes a compiler.
func Example() {
	fmt.Println(cxx.Flags[0])
	// Output: -std=c++17
}
