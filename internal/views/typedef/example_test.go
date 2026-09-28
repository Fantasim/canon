package typedef_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/typedef"
)

// A UInt8's limits are its bounds, and `Int(0..100)` has max 99 (VIEWMODEL.md 12.3 `int`).
func Example() {
	s := typedef.New(context.Background(), typedef.Input{Program: &check.Program{Info: &check.Info{}}}, "a")
	percent := &types.Refined{Of: types.IntType, Range: &types.Bound{Hi: types.Limit{I: 100}, HasLo: true, HasHi: true}}
	for _, t := range []types.Type{types.UInt8Type, percent} {
		b, err := json.Marshal(s.Expr(nil, t))
		fmt.Println(string(b), err)
	}
	// Output:
	// {"kind":"int","bits":8,"signed":false,"min":0,"max":255} <nil>
	// {"kind":"int","bits":64,"signed":true,"min":0,"max":99} <nil>
}
