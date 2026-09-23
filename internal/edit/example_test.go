package edit_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/edit"
)

// A key is a key, never a position, whatever its type; [#n] is a position (API.md P1, P4).
func ExampleParse() {
	p, err := edit.Parse(`resource.farm:farm.modelTypes[3].styles["daily"][#0]`)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(p.Package, p.Root, len(p.Segs), p.Segs[1].Key.Int, p.Segs[3].Key.Text, p.Segs[4].Pos)
	fmt.Println(p)
	// Output: resource.farm farm 5 3 daily 0
	// resource.farm:farm.modelTypes[3].styles["daily"][#0]
}
