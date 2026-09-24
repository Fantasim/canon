package gogen

import (
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// An out-of-range emit mode never panics indexing modeNames; it names it "an unknown mode",
// like kindText does for an out-of-range kind.
func TestModeTextGuardsOutOfRange(t *testing.T) {
	if got := modeText(ir.Mode(99)); got != unknownMode {
		t.Errorf("modeText(99) = %q, want %q", got, unknownMode)
	}
}

// Malformed IR never panics: a negative variant case index is refused, not indexed into the
// cases slice.
func TestVariantExprGuardsNegativeIndex(t *testing.T) {
	v := &ir.Variant{Pkg: "p", Name: "Shape", Cases: []*ir.Case{{Name: "circle", Wire: "circle"}}}
	ct := &types.CaseType{Variant: &types.VariantType{Name: "Shape"}, Index: -1}
	r := &value.Record{T: ct}
	g := &gen{p: &ir.Package{Name: "p"}}
	if got := g.variantExpr(ir.TypeRef{Kind: types.Variant, Named: v}, r); got != nilLit {
		t.Errorf("variantExpr = %q, want %q", got, nilLit)
	}
	if !errors.Is(g.err, ErrMalformed) {
		t.Errorf("g.err = %v, want ErrMalformed", g.err)
	}
}

// go.md §3: a translated fn the plan never named fails cleanly with ErrMalformed, never a nil dereference.
func TestNewPureGuardsMissingPlan(t *testing.T) {
	p := &ir.Package{Name: "p", Emits: []*ir.Emit{{Target: ir.TargetGo, Mode: ir.ModeBaked, GoPackage: "p", GoImport: "example.com/p"}}}
	g := newGen(p, p.Emits[0])
	pr := g.newPure(&ir.ExportFn{Name: "f", Kind: ir.FnTranslated}, "f")
	if pr == nil || pr.plan == nil || pr.plan.Locals == nil {
		t.Fatalf("newPure must return a safe stub pure and plan, got %+v", pr)
	}
	if !errors.Is(g.err, ErrMalformed) {
		t.Errorf("g.err = %v, want ErrMalformed", g.err)
	}
}

// go.md §3: decodeFunc and resolveFunc refuse a class that is no *ir.Case or ir.Type, instead of silently naming it "decode".
func TestClassFuncsGuardUnknownClass(t *testing.T) {
	p := &ir.Package{Name: "p", Emits: []*ir.Emit{{Target: ir.TargetGo, Mode: ir.ModeBaked, GoPackage: "p", GoImport: "example.com/p"}}}
	g := newGen(p, p.Emits[0])
	if got := g.decodeFunc("not a class"); got != "decode" {
		t.Errorf("decodeFunc(unknown) = %q, want %q", got, "decode")
	}
	if !errors.Is(g.err, ErrMalformed) {
		t.Errorf("g.err = %v, want ErrMalformed", g.err)
	}
}

// CODEGEN.md §2.6, §5.2, decision 193: `Retired.` only, or the doc of a case without a type.
func TestKindMemberDoc(t *testing.T) {
	field := []*ir.Field{{Name: "r"}}
	round, nothing := "A round one.", "Nothing."
	cases := []struct {
		c    *ir.Case
		want string
	}{
		{&ir.Case{Doc: round, Fields: field}, ""},
		{&ir.Case{Doc: round, Fields: field, Retired: true}, retiredDoc},
		{&ir.Case{Doc: nothing}, nothing},
		{&ir.Case{Doc: nothing, Retired: true}, nothing + newline + retiredDoc},
		{&ir.Case{Retired: true}, retiredDoc},
	}
	for i, c := range cases {
		if got := kindMemberDoc(c.c); got != c.want {
			t.Errorf("case %d: kindMemberDoc = %q, want %q", i, got, c.want)
		}
	}
}
