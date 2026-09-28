package render_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/views/render"
)

// Two entries of one collection that render the same title show it with their key
// (VIEWMODEL.md S9).
func Example() {
	fmt.Println(render.Disambiguated("Sword", "II_WEA_SWO_WOODEN"))
	// Output:
	// Sword (II_WEA_SWO_WOODEN)
}
