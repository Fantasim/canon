package source_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/source"
)

// A span covers the bytes [Start, End) of one file; End is exclusive.
func Example() {
	s := source.Span{File: 1, Start: 8, End: 13}
	fmt.Println(s.File, s.Start, s.End, s.End-s.Start)
	// Output: 1 8 13 5
}
