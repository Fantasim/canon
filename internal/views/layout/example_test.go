package layout_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/layout"
)

// A record without a view lays every field out in `_other`, then More and Unused fields; a
// deprecated field is only in Unused fields (VIEWMODEL.md L1, L8, L12).
func Example() {
	old := &types.Field{Name: "old", Index: 1, Type: types.IntType, Deprecated: &types.Deprecation{}}
	potion := &types.RecordType{Pkg: "a", Name: "Potion", Fields: []*types.Field{{Name: "heal", Type: types.IntType}, old}}
	index := control.NewIndex(&check.Program{Info: &check.Info{}}, "")
	in := layout.Input{Info: &check.Info{}, Index: index, Res: control.NewResolver(index, control.Env{}), Texts: encode.NewTexts(nil)}
	secs := layout.Section(in, []types.Type{potion})["a.Potion"].Sections
	fmt.Println(secs[0].Kind, secs[0].Fields, secs[1].Kind, secs[2].Kind, secs[2].Fields)
	// Output:
	// other [heal] more unused [old]
}
