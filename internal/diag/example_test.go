package diag_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
)

// Each code is a variable whose Def is its registry entry, generated from spec/ERRORS.md.
func Example() {
	def := diag.E3501.Def()
	v := def.Variants[0]
	fmt.Println(def.Code, def.Severity, def.Package, v.Args[0].Type, v.Args[1].Type, v.Template)
	// Output: E3501 error verify Value Name unknown key {key} in {coll}
}
