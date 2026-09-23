package diagnostics_test

import (
	"fmt"

	"github.com/fantasim/canonlang/tools/audit/internal/lanes/diagnostics"
)

func ExampleNew() {
	l := diagnostics.New()
	fmt.Println(l.Name(), l.Rules())
	// Output: diagnostics [diag-message-inline diag-code-untested diag-code-unreported]
}
