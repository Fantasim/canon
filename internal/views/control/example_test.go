package control_test

import (
	"fmt"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
)

// In a table cell a segmented choice is a select, and its optional wrapper clears (VIEWMODEL.md
// C46); `slider` is a hint a Bool does not take (C40).
func Example() {
	cell := control.Cell(vm.Control{Kind: control.CtlSegmented, Optional: &vm.ControlOptional{Unset: "segment"}})
	fmt.Println(cell.Kind, cell.Optional.Unset)
	fmt.Println(control.Accepts(control.CtlSlider, types.BoolType))
	// Output:
	// select clear
	// true false
}
