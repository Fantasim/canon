package ir_test

import (
	"fmt"
	"regexp"

	"github.com/fantasim/canonlang/internal/ir"
)

// An input's pattern, for std::regex_search over UTF-8 bytes: `é` becomes its two bytes, and
// `\d` an explicit class no locale can widen.
func ExampleCppPattern() {
	s, err := ir.CppPattern(regexp.MustCompile(`^é-\d+$`))
	fmt.Println(s, err)
	// Output: ^\xc3\xa9\x2d[0-9]+$ <nil>
}
