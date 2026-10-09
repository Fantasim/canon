package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// TestOpenEnum is DECISIONS 339: an enum is open in a go types emit whose `open` names it, its own package's or, for another package's enum, the go emit of that package as the importer sees it; never in another mode or target.
func TestOpenEnum(t *testing.T) {
	own := &ir.Enum{Pkg: "a", Name: "Dst"}
	far := &ir.Enum{Pkg: "b", Name: "Far"}
	opened := &ir.Emit{Target: ir.TargetGo, Mode: ir.ModeTypes, Open: []string{"Dst"}}
	farEmit := &ir.Emit{Target: ir.TargetGo, Mode: ir.ModeTypes, Open: []string{"Far"}}
	p := &ir.Package{Name: "a", Imports: []*ir.PackageRef{{Name: "b", Emits: []*ir.Emit{farEmit}}}}
	cases := []struct {
		name string
		e    *ir.Emit
		en   *ir.Enum
		want bool
	}{
		{"own, opened", opened, own, true},
		{"own, in baked mode", &ir.Emit{Target: ir.TargetGo, Mode: ir.ModeBaked, Open: []string{"Dst"}}, own, false},
		{"own, not listed", &ir.Emit{Target: ir.TargetGo, Mode: ir.ModeTypes}, own, false},
		{"own, a cpp emit", &ir.Emit{Target: ir.TargetCpp, Mode: ir.ModeTypes, Open: []string{"Dst"}}, own, false},
		{"another package's, opened there", &ir.Emit{Target: ir.TargetGo, Mode: ir.ModeBaked}, far, true},
	}
	for _, c := range cases {
		if got := ir.OpenEnum(p, c.e, c.en); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	farEmit.Mode = ir.ModeData
	if ir.OpenEnum(p, opened, far) {
		t.Error("an enum its package's data emit lists is open")
	}
}

// TestTextDecoded is DECISIONS 340: a @text fn whose result, without a maybe-file's `?`, is a map, a list or a keyed list of public types has a Decode<Fn>File; a String or record file, a list of a local record, of optional elements or of Never has none, and is never refused.
func TestTextDecoded(t *testing.T) {
	public := &ir.Record{Pkg: "a", Name: "Point"}
	local := &ir.Record{Pkg: "a", Name: "Hidden"}
	listOf := func(r *ir.Record) ir.TypeRef {
		return ir.TypeRef{Kind: types.List, Elem: &ir.TypeRef{Kind: types.Record, Named: r}}
	}
	mapT := ir.TypeRef{Kind: types.Map, Key: &ir.TypeRef{Kind: types.String}, Elem: &ir.TypeRef{Kind: types.Int}}
	maybe := ir.TypeRef{Kind: types.Optional, Elem: &mapT}
	fns := []*ir.ExportFn{
		{Name: "points", Result: listOf(public)}, {Name: "hidden", Result: listOf(local)},
		{Name: "counts", Result: maybe}, {Name: "name", Result: ir.TypeRef{Kind: types.String}},
		{Name: "point", Result: ir.TypeRef{Kind: types.Record, Named: public}},
		{Name: "holes", Result: ir.TypeRef{Kind: types.List, Elem: &ir.TypeRef{Kind: types.Optional, Elem: &ir.TypeRef{Kind: types.Int}}}},
		{Name: "nothing", Result: ir.TypeRef{Kind: types.List, Elem: &ir.TypeRef{Kind: types.Never}}},
	}
	p := &ir.Package{Name: "a", Types: []ir.Type{public}, TextFns: fns}
	got := ir.TextDecoded(p, &ir.Emit{Target: ir.TargetGo, Mode: ir.ModeTypes})
	if len(got) != 2 || got[0].Name != "points" || got[1].Name != "counts" {
		t.Fatalf("decoded: %v", got)
	}
	if r := ir.TextResult(got[1]); r.Kind != types.Map {
		t.Errorf("a maybe-file's result: %v, want its map", r.Kind)
	}
	e := &ir.Emit{Target: ir.TargetGo, Mode: ir.ModeTypes, GoPackage: "a", GoImport: "example.com/a"}
	if name := ir.PlanGoNames(p, e).TextDecoder(fns[2]); name != "DecodeCountsFile" {
		t.Errorf("TextDecoder: %s, want DecodeCountsFile", name)
	}
	e.Mode = ir.ModeBaked
	if n := len(ir.PlanGoNames(p, e).TextDecoders()); n != 0 {
		t.Errorf("a baked emit writes %d text decoders", n)
	}
}
