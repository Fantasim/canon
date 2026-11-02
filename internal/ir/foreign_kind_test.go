package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// TestForeignUsesKindConstant is CODEGEN.md §4.4, §5.1, DECISIONS 438: a constant of another package's variant kind builds none of that package's classes, so ForeignUses lists neither a read nor a write of the variant.
func TestForeignUsesKindConstant(t *testing.T) {
	reward := &ir.Variant{Pkg: "lib", Name: "Reward", Cases: []*ir.Case{{Name: "gold"}}}
	kind := ir.TypeRef{Kind: types.VariantKind, Named: reward}
	list := ir.TypeRef{Kind: types.List, Elem: &kind}
	p := &ir.Package{Name: "app", Consts: []*ir.Const{{Name: "BK", Type: kind}, {Name: "BKS", Type: list}}}
	for _, mode := range []ir.Mode{ir.ModeBaked, ir.ModeData, ir.ModeTypes} {
		u := ir.ForeignUses(p, &ir.Emit{Target: ir.TargetCpp, Mode: mode})
		if len(u.Read) != 0 || len(u.Written) != 0 {
			t.Errorf("mode %d: read %v, written %v, want none", mode, u.Read, u.Written)
		}
	}
}
