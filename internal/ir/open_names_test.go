package ir_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// TestOpenAndFinderNames is CODEGEN.md §3.3, §5.2, §5.9 and DECISIONS 362, 363, 369: the names the printers read off the plans. An opened enum Dst gives TypeScript DstKnown, isDstKnown and dstName; the C++ plan lists the package's opened enums; a decoded @text fn items gives C++ ItemsFile; a @stable field foo_bar gives TypeScript findByFooBar, whatever its @ts(name:).
func TestOpenAndFinderNames(t *testing.T) {
	dst := &ir.Enum{Pkg: "a", Name: "Dst"}
	closed := &ir.Enum{Pkg: "a", Name: "Tone"}
	if got := ir.TSOpenEnumNames(dst); got != (ir.TSOpenNames{Known: "DstKnown", IsKnown: "isDstKnown", Name: "dstName"}) {
		t.Errorf("TS open names %+v", got)
	}
	e := &ir.Emit{Target: ir.TargetCpp, Mode: ir.ModeTypes, Namespace: "a", Open: []string{"Dst"}}
	pl := ir.PlanCppNames(&ir.Package{Name: "a", Types: []ir.Type{dst, closed}, Emits: []*ir.Emit{e}}, e)
	if got := pl.OpenEnums(); !slices.Equal(got, []*ir.Enum{dst}) {
		t.Errorf("C++ opened enums %v, want [Dst]", got)
	}
	if got := pl.TextFile(&ir.ExportFn{Name: "items"}); got != "ItemsFile" {
		t.Errorf("C++ text file %q, want ItemsFile", got)
	}
	f := &ir.Field{Name: "foo_bar", Type: ir.TypeRef{Kind: types.Int}, Stable: true, TS: ir.NameOptions{Name: "code"}}
	if got := ir.TSFinderName(f); got != "findByFooBar" {
		t.Errorf("TS finder %q, want findByFooBar", got)
	}
}
