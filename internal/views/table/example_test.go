package table_test

import (
	"encoding/json"
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/table"
)

// A keyed list of records without a view is a table keyed by its key field, whose first column
// is the entry and whose other columns are its scalar fields but the key (VIEWMODEL.md T1, T6, T7).
func Example() {
	id := &types.Field{Name: "id", Type: types.StringType}
	potion := &types.RecordType{Pkg: "a", Name: "Potion", Fields: []*types.Field{id, {Name: "heal", Index: 1, Type: types.IntType}}}
	index := control.NewIndex(&check.Program{Info: &check.Info{}}, "")
	tables := table.New(index, encode.NewTexts(nil))
	res := control.NewResolver(index, control.Env{Table: tables.Complete})
	tables.Bind(res)
	out, err := json.Marshal(res.Value(nil, &types.ListType{Elem: potion, KeyedBy: id}))
	fmt.Println(string(out), err)
	// Output:
	// {"kind":"table","of":"a.Potion","key":"id","orderable":true,"columns":[{"field":"$entry","mode":"entry"},{"field":"heal","mode":"edit","cell":{"kind":"number"}}]} <nil>
}
