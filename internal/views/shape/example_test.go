package shape_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// A sized integer has both bounds; Int, whose limits are int64's, has none (VIEWMODEL.md C12).
func ExampleBounds() {
	fmt.Println(shape.Bounds(types.Int8Type))
	fmt.Println(shape.Bounds(types.IntType))
	// Output:
	// true true
	// false false
}
