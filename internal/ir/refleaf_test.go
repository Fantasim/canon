package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// leafRef is a ref keyed by key.
func leafRef(key ir.TypeRef) ir.TypeRef { return ir.TypeRef{Kind: types.Ref, Key: &key} }

// leafTone is an enum whose second member is written "z2" on the wire.
func leafTone() *ir.Enum {
	return &ir.Enum{Name: "Tone", Members: []*ir.EnumMember{{Name: "first", Wire: "first"}, {Name: "second", Wire: "z2", Index: 1}}}
}

// CODEGEN.md §5.9, DECISIONS 456: RefLeaf is the innermost ref of a chain of refs, the ref whose key is no ref; a type that is no ref, or a ref of no key, is itself.
func TestRefLeaf(t *testing.T) {
	t.Parallel()
	str := ir.TypeRef{Kind: types.String}
	one := leafRef(str)
	two := leafRef(one)
	three := leafRef(two)
	cases := []struct {
		name string
		in   ir.TypeRef
		want ir.TypeRef
	}{
		{"not a ref", str, str},
		{"a ref of no key", ir.TypeRef{Kind: types.Ref}, ir.TypeRef{Kind: types.Ref}},
		{"one level", one, one},
		{"two levels", two, one},
		{"three levels", three, one},
	}
	for _, c := range cases {
		got := ir.RefLeaf(c.in)
		if got.Kind != c.want.Kind || got.Key != c.want.Key {
			t.Errorf("%s: RefLeaf = %+v, want %+v", c.name, got, c.want)
		}
	}
}

// CODEGEN.md §5.9, WIRE.md §5.9, DECISIONS 387, 456: RefKeyEnum and KeyWire read the key that ends a chain of refs: the enum, and its member's wire.
func TestRefKeyEnumAndKeyWireDownAChain(t *testing.T) {
	t.Parallel()
	tone := leafTone()
	one := leafRef(ir.TypeRef{Kind: types.Enum, Named: tone})
	two := leafRef(one)
	three := leafRef(two)
	for name, ref := range map[string]ir.TypeRef{"one": one, "two": two, "three": three} { //canon:unordered each ref is checked alone
		e, ok := ir.RefKeyEnum(ref)
		if !ok || e != tone {
			t.Errorf("%s: RefKeyEnum = %v, %v, want the enum", name, e, ok)
		}
		if got := ir.KeyWire(ref, value.Key{S: "second"}); got != "z2" {
			t.Errorf("%s: KeyWire = %q, want z2", name, got)
		}
	}
	if _, ok := ir.RefKeyEnum(leafRef(leafRef(ir.TypeRef{Kind: types.String}))); ok {
		t.Error("RefKeyEnum finds an enum at the end of a chain of String")
	}
}
