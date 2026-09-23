package threshold_test

import (
	"fmt"

	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

func ExampleSet_Expand() {
	s := threshold.Set{FnLines: 60, FnStatements: 40}
	text, err := s.Expand("function body over {fn-lines} lines or {fn-statements} statements")
	fmt.Println(text, err)
	// Output: function body over 60 lines or 40 statements <nil>
}
