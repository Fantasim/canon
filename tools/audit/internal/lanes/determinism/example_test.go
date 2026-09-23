package determinism_test

import (
	"fmt"

	"github.com/fantasim/canonlang/tools/audit/internal/lanes/determinism"
)

func ExampleNew() {
	l := determinism.New()
	fmt.Println(l.Name(), l.Rules())
	// Output: determinism [maprange]
}
