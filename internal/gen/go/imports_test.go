package gogen_test

import (
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// A standard-library import is classified by a fixed name set, not by whether its path has a dot.
func TestImportGroupingByOrigin(t *testing.T) {
	other := &ir.Enum{Pkg: "q", Name: "Q", Members: []*ir.EnumMember{{Name: "x", Wire: "x"}}}
	p := pkg(enum("E", "x"), &ir.Record{Pkg: "p", Name: "R", Fields: []*ir.Field{
		{Name: "q", Type: ir.TypeRef{Kind: types.Enum, Named: other}},
	}})
	p.Imports = []*ir.PackageRef{{Name: "q", Emits: []*ir.Emit{{Target: ir.TargetGo, GoImport: "localmodule", GoPackage: "q"}}}}
	files, err := gogen.Generate(p, p.Emits[0])
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	src := string(files[1].Content)
	if !strings.Contains(src, "import (\n\t\"iter\"\n\t\"strconv\"\n\n\tq \"localmodule\"\n)") {
		t.Errorf("localmodule (no dot) should be grouped with the others, not with iter and strconv:\n%s", src)
	}
}
