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

// A list field written `@json(bits)` is a set (VIEWMODEL.md C32).
func ExampleTypes_FieldExpr() {
	s := typedef.New(context.Background(), typedef.Input{Program: &check.Program{Info: &check.Info{}}}, "a")
	flags := &types.Field{Name: "flags", Type: &types.ListType{Elem: types.BoolType}, Enc: types.EncBits}
	field, err := json.Marshal(s.FieldExpr(nil, flags))
	alone, err2 := json.Marshal(s.Expr(nil, flags.Type))
	fmt.Println(string(field), string(alone), err, err2)
	// Output: {"kind":"list","of":{"kind":"bool"},"unique":true} {"kind":"list","of":{"kind":"bool"}} <nil> <nil>
}
