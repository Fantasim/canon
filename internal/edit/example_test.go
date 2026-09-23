package edit_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
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

// A resolved path keeps, per segment, the container it was read in and the value there.
func ExampleResolved() {
	p, err := edit.Parse("p:levels[1]")
	if err != nil {
		fmt.Println(err)
		return
	}
	levels := &types.ListType{Elem: types.IntType}
	second := &value.Int{V: 20, T: types.IntType}
	r := edit.Resolved{Canonical: p.String(), Steps: []edit.Step{{Seg: p.Segs[0], Container: levels, Value: second}}, Target: second}
	for _, s := range r.Steps {
		fmt.Println(r.Canonical, s.Seg.Key.Int, s.Container, s.Value.CanonText(), r.Target == s.Value)
	}
	// Output: p:levels[1] 1 [Int] 20 true
}
