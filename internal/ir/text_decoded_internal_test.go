package ir

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// TestOwnerHooksMatchesPlan is DECISIONS 340 and 363 with CODEGEN.md §5.14: ownerHooks, which an importer's generator decides from the owner's emit of its target, equals the owner's own plan (planWrites, which stage E's ForeignResolvedRef asks), through the NoHook stage E fills from it, in every mode of go and cpp: a record the target table itself holds (Linked), one without a ref (Flat), one whose ref targets an unselected value (Far), one holding a ref into a selected table but held by another value (Listed) or by none (Loose), and each case of a variant Step held by a selected table, its case hop holding a ref into that table, which the loader resolves (NoHook's Variant.Case).
func TestOwnerHooksMatchesPlan(t *testing.T) {
	ref := func(value string) TypeRef {
		return TypeRef{Kind: types.Ref, Ref: &RefTarget{Coll: types.CollLet, Pkg: "e", Value: value}, Key: &TypeRef{Kind: types.String}}
	}
	field := func(name string, t TypeRef) *Field { return &Field{Name: name, WirePath: []string{name}, Type: t} }
	linked := &Record{Pkg: "e", Name: "Linked", Fields: []*Field{field("next", ref("links"))}}
	listed := &Record{Pkg: "e", Name: "Listed", Fields: []*Field{field("all", TypeRef{Kind: types.List, Elem: ptr(ref("links"))})}}
	flat := &Record{Pkg: "e", Name: "Flat", Fields: []*Field{field("n", TypeRef{Kind: types.Int, Signed: true})}}
	far := &Record{Pkg: "e", Name: "Far", Fields: []*Field{field("to", ref("others"))}}
	loose := &Record{Pkg: "e", Name: "Loose", Fields: []*Field{field("to", ref("links"))}}
	hop := &Case{Name: "hop", Wire: "hop", Fields: []*Field{field("to", ref("paths"))}}
	rest := &Case{Name: "rest", Wire: "rest", Fields: []*Field{field("n", TypeRef{Kind: types.Int, Signed: true})}}
	step := &Variant{Pkg: "e", Name: "Step", Tag: "kind", Cases: []*Case{hop, rest}}
	path := &Record{Pkg: "e", Name: "Path", Fields: []*Field{field("step", TypeRef{Kind: types.Variant, Named: step})}}
	table := func(r *Record) TypeRef {
		return TypeRef{Kind: types.Table, Elem: &TypeRef{Kind: types.Record, Named: r}}
	}
	owner := &Package{Name: "e", Types: []Type{linked, listed, flat, far, loose, step, path}, Values: []*Value{
		{Name: "links", Type: table(linked)}, {Name: "lists", Type: table(listed)}, {Name: "flats", Type: table(flat)},
		{Name: "fars", Type: table(far)}, {Name: "others", Type: table(flat)}, {Name: "paths", Type: table(path)},
	}}
	importer := &Package{Name: "c"}
	use := &ForeignUse{variantOf: map[*Case]*Variant{hop: step, rest: step}}
	caseUnwritten := false
	for _, e := range []*Emit{
		{Target: TargetGo, Mode: ModeData}, {Target: TargetGo, Mode: ModeBaked}, {Target: TargetGo, Mode: ModeTypes},
		{Target: TargetCpp, Mode: ModeData}, {Target: TargetCpp, Mode: ModeBaked}, {Target: TargetCpp, Mode: ModeTypes},
	} {
		e.Values, e.GoPackage, e.GoImport, e.Namespace = []string{"links", "lists", "flats", "fars", "paths"}, "e", "example.com/e", "e"
		owner.Emits = []*Emit{e}
		importer.Imports = []*PackageRef{{Name: "e", Emits: owner.Emits}}
		written := planWrites(owner, e)
		if e.Mode == ModeData {
			e.NoHook = unwritten(owner, written)
		}
		for _, r := range []*Record{linked, listed, flat, far, loose} {
			if got, want := ownerHooks(importer, e.Target, use, r), written(r, nil); got != want {
				t.Errorf("%s, target %d mode %d: ownerHooks %v, the owner's plan %v", r.Name, e.Target, e.Mode, got, want)
			}
		}
		for _, c := range step.Cases {
			caseUnwritten = caseUnwritten || !written(step, c)
			if got, want := ownerHooks(importer, e.Target, use, c), written(step, c); got != want {
				t.Errorf("Step.%s, target %d mode %d: ownerHooks %v, the owner's plan %v", c.Name, e.Target, e.Mode, got, want)
			}
		}
		if got, want := ownerHooks(importer, e.Target, use, step), written(step, hop) && written(step, rest); got != want {
			t.Errorf("Step, target %d mode %d: ownerHooks %v, its cases' %v", e.Target, e.Mode, got, want)
		}
	}
	if !caseUnwritten {
		t.Error("no case of Step goes without a hook: NoHook's Variant.Case form is not exercised")
	}
}

func ptr(t TypeRef) *TypeRef { return &t }
