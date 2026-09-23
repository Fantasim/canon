package jsongen_test

import (
	"fmt"
	"strings"

	jsongen "github.com/fantasim/canonlang/internal/gen/json"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Directory mode: one `<value>.json` per public value, each with its `$schema`.
func Example() {
	role := &types.EnumType{Pkg: "sovcommon.roles", Name: "Role", Members: []*types.Member{
		{Name: "member", Wire: "member"}, {Name: "admin", Wire: "admin", Index: 1},
	}}
	roleIR := &ir.Enum{Pkg: "sovcommon.roles", Name: "Role", Members: []*ir.EnumMember{
		{Name: "member", Wire: "member"}, {Name: "admin", Wire: "admin", Index: 1},
	}}
	p := &ir.Package{Name: "teamboard", Values: []*ir.Value{{
		Name: "triageMinRole", Type: ir.TypeRef{Kind: types.Enum, Named: roleIR}, V: &value.Member{Enum: role, Index: 1},
	}}}
	e := &ir.Emit{Target: ir.TargetJSON, Out: "@sovcommon/teamboard/data/", Dir: "sovcommon/teamboard/data"}
	var gen ir.Generator = jsongen.Generate
	files, err := gen(p, e)
	fmt.Println(files[0].Path, err, strings.Split(string(files[0].Content), "\n")[1:3])
	// Output: triageMinRole.json <nil> [  "$schema": "sovcommon.roles.Role@9e4c5be5",   "value": "admin"]
}
