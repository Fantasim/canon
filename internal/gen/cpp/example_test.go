package cppgen_test

import (
	"fmt"
	"strings"

	cppgen "github.com/fantasim/canonlang/internal/gen/cpp"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// A data-mode cpp emit writes the runtime headers, the package header and its source; the
// loader of a keyed list is a static Load on its container.
func ExampleGenerate() {
	cup := &ir.Record{Pkg: "shop", Name: "Cup", Fields: []*ir.Field{
		{Name: "size", WirePath: []string{"size"}, Type: ir.TypeRef{Kind: types.String}},
	}}
	elem := ir.TypeRef{Kind: types.Record, Named: cup}
	cups := &ir.Value{Name: "cups", Schema: "shop.Cup@00000001", Type: ir.TypeRef{
		Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "size", WirePath: []string{"size"}},
	}}
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "shop/out", Mode: ir.ModeData, Namespace: "shop"}
	p := &ir.Package{Name: "shop", Dir: "shop", Types: []ir.Type{cup}, Values: []*ir.Value{cups}, Emits: []*ir.Emit{emit}}
	files, err := cppgen.Generate(p, emit)
	if err != nil {
		fmt.Println(err)
		return
	}
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	fmt.Println(strings.Join(paths, " "), strings.Contains(string(files[2].Content), "static std::shared_ptr<const Cups> Load("))
	// Output: canon_runtime.h canon_runtime_json.h shop.gen.h shop.gen.cpp true
}
