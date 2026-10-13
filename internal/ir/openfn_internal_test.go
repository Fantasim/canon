package ir

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// TestReadsOpenEnum is DECISIONS 365: a translated fn takes or reads an opened enum wherever its parameter or read type holds one, through a list, a map key, a record or a variant's case; a type holding none, or a closed enum, does not.
func TestReadsOpenEnum(t *testing.T) {
	dst := &Enum{Pkg: "a", Name: "Dst"}
	tone := &Enum{Pkg: "a", Name: "Tone"}
	enumOf := func(e *Enum) *TypeRef { return &TypeRef{Kind: types.Enum, Named: e} }
	holder := &Record{Pkg: "a", Name: "Holder", Fields: []*Field{{Name: "dst", Type: *enumOf(dst)}}}
	hop := &Case{Name: "hop", Fields: []*Field{{Name: "dst", Type: *enumOf(dst)}}}
	step := &Variant{Pkg: "a", Name: "Step", Cases: []*Case{hop}}
	e := &Emit{Target: TargetTS, Mode: ModeTypes, Open: []string{"Dst"}}
	p := &Package{Name: "a", Types: []Type{dst, tone, holder, step}, Emits: []*Emit{e}}
	cases := []struct {
		name string
		t    TypeRef
		want bool
	}{
		{"the enum", *enumOf(dst), true},
		{"a list of it", TypeRef{Kind: types.List, Elem: enumOf(dst)}, true},
		{"a map keyed by it", TypeRef{Kind: types.Map, Key: enumOf(dst), Elem: &TypeRef{Kind: types.Int}}, true},
		{"a record holding it", TypeRef{Kind: types.Record, Named: holder}, true},
		{"a variant whose case holds it", TypeRef{Kind: types.Variant, Named: step}, true},
		{"a case holding it", TypeRef{Kind: types.Case, Named: step, Case: hop}, true},
		{"a closed enum", *enumOf(tone), false},
		{"an Int", TypeRef{Kind: types.Int}, false},
	}
	for _, c := range cases {
		byParam := &ExportFn{Kind: FnTranslated, Params: []*Param{{Name: "x", Type: c.t}}}
		byRead := &ExportFn{Kind: FnTranslated, Reads: []*Read{{Name: "x", Type: c.t}}}
		if got := readsOpenEnum(p, e, byParam); got != c.want {
			t.Errorf("a parameter of %s: %v, want %v", c.name, got, c.want)
		}
		if got := readsOpenEnum(p, e, byRead); got != c.want {
			t.Errorf("a read of %s: %v, want %v", c.name, got, c.want)
		}
	}
}
