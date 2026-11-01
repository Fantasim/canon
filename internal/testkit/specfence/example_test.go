package specfence_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/specfence"
)

// Example reads the block of a language that follows a heading.
func Example() {
	doc := "## 7.4 Header\n\n```cpp\nint x;\n```\n"
	block, err := specfence.Find(doc, "## 7.4", "cpp")
	fmt.Printf("%q %v\n", block, err)
	// Output: "int x;\n" <nil>
}
