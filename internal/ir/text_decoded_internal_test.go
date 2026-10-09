package ir

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// TestOwnerHooksMatchesPlan is DECISIONS 340 with CODEGEN.md §5.14: ownerHooks, which an importer's generator decides from the owner's go emit alone, never says a hook is written where the owner's own plan (planWrites, which stage E's ForeignResolvedRef asks) writes none, so a decoded @text result is never refused; it agrees exactly for a record the target table itself holds (Linked), one without a ref (Flat) and one whose ref targets an unselected value (Far). It is conservative where only the owner's values tell: a record holding a ref into a selected table but held by another value (Listed) or by none (Loose) keeps its hook, which ownerHooks cannot see from the importer's IR, so such a result goes without a decoder.
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
	table := func(r *Record) TypeRef {
		return TypeRef{Kind: types.Table, Elem: &TypeRef{Kind: types.Record, Named: r}}
	}
	owner := &Package{Name: "e", Types: []Type{linked, listed, flat, far, loose}, Values: []*Value{
		{Name: "links", Type: table(linked)}, {Name: "lists", Type: table(listed)}, {Name: "flats", Type: table(flat)},
		{Name: "fars", Type: table(far)}, {Name: "others", Type: table(flat)},
	}}
	importer := &Package{Name: "c"}
	for _, mode := range []Mode{ModeData, ModeBaked, ModeTypes} {
		e := &Emit{Target: TargetGo, Mode: mode, Values: []string{"links", "lists", "flats", "fars"}, GoPackage: "e", GoImport: "example.com/e"}
		owner.Emits = []*Emit{e}
		importer.Imports = []*PackageRef{{Name: "e", Emits: owner.Emits}}
		written := planWrites(owner, e)
		for _, r := range []*Record{linked, listed, flat, far, loose} {
			got, want := ownerHooks(importer, "e", r), written(r, nil)
			if got && !want || r != loose && r != listed && got != want {
				t.Errorf("%s in mode %d: ownerHooks %v, the owner's plan %v", r.Name, mode, got, want)
			}
		}
	}
}

func ptr(t TypeRef) *TypeRef { return &t }
