package tsgen_test

import (
	"fmt"
	"strings"

	tsgen "github.com/fantasim/canonlang/internal/gen/ts"
	"github.com/fantasim/canonlang/internal/ir"
)

// A baked ts emit writes the package's module: its enums as unions with their Members, Names and Index tables.
func ExampleGenerate() {
	size := &ir.Enum{Pkg: "shop", Name: "Size", Doc: "Cup sizes.", Members: []*ir.EnumMember{
		{Name: "small", Wire: "small"}, {Name: "extra_large", Wire: "XL", Index: 1},
	}}
	p := &ir.Package{
		Name: "shop", Dir: "shop", Types: []ir.Type{size},
		Emits: []*ir.Emit{{Target: ir.TargetTS, Dir: "web", FileName: "shop.ts", Mode: ir.ModeBaked}},
	}
	files, err := tsgen.Generate(p, p.Emits[0])
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(files[0].Path, strings.Contains(string(files[0].Content), `export type Size = "small" | "XL";`))
	// Output: shop.ts true
}
