package gogen_test

import (
	"fmt"
	"strings"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// A baked go emit writes the package's main file and its rt helper package.
func ExampleGenerate() {
	size := &ir.Enum{Pkg: "shop", Name: "Size", Doc: "Cup sizes.", Members: []*ir.EnumMember{
		{Name: "small", Wire: "small"}, {Name: "extra_large", Wire: "XL", Index: 1},
	}}
	p := &ir.Package{
		Name: "shop", Dir: "shop", Doc: "Coffee.", Types: []ir.Type{size},
		Consts: []*ir.Const{{Name: "DEFAULT_SIZE", Type: ir.TypeRef{Kind: types.Enum, Named: size}, V: &value.Member{Index: 1}}},
		Emits:  []*ir.Emit{{Target: ir.TargetGo, Dir: "shop/out", GoImport: "example.com/shop", Mode: ir.ModeBaked, GoPackage: "shop"}},
	}
	files, err := gogen.Generate(p, p.Emits[0])
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(files[0].Path, files[1].Path, strings.Contains(string(files[1].Content), "const DefaultSize = SizeExtraLarge"))
	// Output: rt/rt.go shop.gen.go true
}
