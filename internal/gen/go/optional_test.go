package gogen_test

import (
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// mapT is a map type.
func mapT(k, v ir.TypeRef) ir.TypeRef { return ir.TypeRef{Kind: types.Map, Key: &k, Elem: &v} }

// optionalFields are [Int?], [String?], [Cell?], {String: Int?} and {Mood: Cell?} as fields of Thing (CODEGEN.md §4.2).
func optionalFields(cell, mood ir.Type) []*ir.Field {
	return []*ir.Field{
		wired("ints", "ints", "", listT(optT(intT))),
		wired("words", "words", "", listT(optT(strT))),
		wired("cells", "cells", "", listT(optT(typed(cell, types.Record)))),
		wired("counts", "counts", "", mapT(strT, optT(intT))),
		wired("byMood", "byMood", "", mapT(typed(mood, types.Enum), optT(typed(cell, types.Record)))),
	}
}

// generated is the text of the one .gen.go file Generate writes for p's first emit in mode.
func generated(t *testing.T, p *ir.Package, mode ir.Mode) string {
	t.Helper()
	e := *p.Emits[0]
	e.Mode = mode
	files, err := gogen.Generate(p, &e)
	if err != nil {
		t.Fatalf("%v: %v", mode, err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".gen.go") {
			return string(f.Content)
		}
	}
	t.Fatalf("%v: no .gen.go file", mode)
	return ""
}

// DECISIONS 385, CODEGEN.md §4.2: Go holds [T?] as rt.List[*T] and {K: V?} as rt.Map[K, *V], a record or variant element being [R]'s own pointer, in data and types mode; a loader and a decoder read null as nil, a present scalar through rt.Ptr.
func TestOptionalElementsGenerate(t *testing.T) {
	for _, mode := range []ir.Mode{ir.ModeData, ir.ModeTypes} {
		p := dataThing()
		cell := &ir.Record{Pkg: "demo", Name: "Cell", Fields: []*ir.Field{wired("n", "n", "", intT)}}
		mood := &ir.Enum{Pkg: "demo", Name: "Mood", Members: []*ir.EnumMember{{Name: "calm", Wire: "calm"}, {Name: "angry", Wire: "angry", Index: 1}}}
		p.Types = append(p.Types, cell, mood)
		for _, f := range optionalFields(cell, mood) {
			addField(p, f)
		}
		src := generated(t, p, mode)
		for _, want := range []string{
			"ints   rt.List[*int64]", "words  rt.List[*string]", "cells  rt.List[*Cell]", "counts rt.Map[string, *int64]", "byMood rt.Map[Mood, *Cell]",
			`if string(x`, `!= "null" {`, "rt.Ptr[int64](", "rt.Ptr[string](",
		} {
			if !strings.Contains(src, want) {
				t.Errorf("%v: no %q in\n%s", mode, want, src)
			}
		}
	}
}

// DECISIONS 387, CODEGEN.md §4.1, §5.8: a data loader checks a literal union over a ref after the load, where a plain ref of it is resolved, accepting a literal as it is; a types-mode decoder reads the string unchecked (§5.13).
func TestRefUnionGenerate(t *testing.T) {
	rec := &ir.Record{Pkg: "demo", Name: "Thing"}
	elem := typed(rec, types.Record)
	union := ir.TypeRef{Kind: types.LitUnion, Elem: &ir.TypeRef{Kind: types.Ref, Key: &strT, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: "demo", Value: "ts", Elem: rec}}, Literals: []string{"none"}}
	rec.Fields = []*ir.Field{wired("u", "u", "", union), wired("us", "us", "", listT(optT(union))), wired("by", "by", "", mapT(union, intT))}
	p := &ir.Package{Name: "demo", Dir: "demo", Types: []ir.Type{rec}, Emits: []*ir.Emit{goData("demo", "demo")}, Values: []*ir.Value{
		{Name: "ts", Schema: "demo.Thing@00000001", IDs: []string{"x"}, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}},
	}}
	src := generated(t, p, ir.ModeData)
	for _, want := range []string{
		`if out.u != "none" {`, `ctx.rows.Find(ThingID(out.u))`, `u: no entry %v`,
		`(*out.us.At(`, `us[%d]: no entry %v`, `by.`, `kp`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("data: no %q in\n%s", want, src)
		}
	}
	if strings.Contains(generated(t, p, ir.ModeTypes), "no entry") {
		t.Error("types: a decoder checked a union's key")
	}
}

// DECISIONS 385, 387, CODEGEN.md §4.1, §4.2: a baked emit writes none as nil and a present scalar through the typed rt.Ptr, a literal union over a ref as the key's wire text.
func TestBakedOptionalConstants(t *testing.T) {
	p := &ir.Package{Name: "demo", Dir: "demo", Emits: []*ir.Emit{goData("demo", "demo")}}
	rec := &ir.Record{Pkg: "demo", Name: "Thing"}
	union := ir.TypeRef{Kind: types.LitUnion, Elem: &ir.TypeRef{Kind: types.Ref, Key: &strT, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: "demo", Value: "ts", Elem: rec}}, Literals: []string{"none"}}
	p.Consts = []*ir.Const{
		{Name: "XS", Type: listT(optT(intT)), V: &value.List{Elems: []value.Value{&value.Int{V: 1}, &value.None{}}}},
		{Name: "BY", Type: mapT(strT, optT(strT)), V: &value.Map{Keys: []value.Value{&value.Str{V: "a"}, &value.Str{V: "b"}}, Vals: []value.Value{&value.None{}, &value.Str{V: "w"}}}},
		{Name: "PICKS", Type: listT(union), V: &value.List{Elems: []value.Value{&value.Ref{Key: value.Key{S: "sword"}}, &value.Str{V: "none"}}}},
	}
	src := generated(t, p, ir.ModeBaked)
	for _, want := range []string{
		"func Xs() rt.List[*int64] { return rt.MakeList([]*int64{rt.Ptr[int64](1), nil}) }",
		`return rt.MakeMap([]string{"a", "b"}, []*string{nil, rt.Ptr[string]("w")})`,
		`func Picks() rt.List[string] { return rt.MakeList([]string{"sword", "none"}) }`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("no %q in\n%s", want, src)
		}
	}
}
