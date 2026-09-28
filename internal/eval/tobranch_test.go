package eval_test

import (
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// TYPES.md §6.2, §11.6: a symbol becomes a member, case or key; only a written literal converts.
func TestToBranch(t *testing.T) {
	color := &types.EnumType{Pkg: "a", Name: "Color", Members: []*types.Member{{Name: "red"}, {Name: "blue", Index: 1}}}
	shape := &types.VariantType{Pkg: "a", Name: "Shape"}
	shape.Cases = []*types.CaseType{
		{Variant: shape, Name: "dot"},
		{Variant: shape, Name: "box", Index: 1, Fields: []*types.Field{{Name: "w", Type: types.IntType}}},
	}
	tags := &types.RefType{Target: &types.Collection{Kind: types.CollLet, Pkg: "a", Name: "tags"}}
	sym := func(name string) value.Value { return &value.Symbol{Name: name} }
	cases := []struct {
		name    string
		v       value.Value
		t       types.Type
		written bool
		want    string
	}{
		{"a member", sym("blue"), color, false, "*value.Member blue 0"},
		{"no member", sym("green"), color, false, "*value.Symbol green 1"},
		{"a case without fields", sym("dot"), shape, false, "*value.Record dot 0"},
		{"a case with fields", sym("box"), shape, false, "*value.Record box{w: <nil>} 2"},
		{"a key", sym("red"), tags, false, "*value.Ref red 0"},
		{"a written string key", &value.Str{V: "red", T: types.StringType}, tags, true, "*value.Ref red 0"},
		{"a computed string key", &value.Str{V: "red", T: types.StringType}, tags, false, "*value.Str red 1"},
		{"an integer on string keys", &value.Int{V: 1, T: types.IntType}, tags, true, "*value.Int 1 1"},
		{"a written integer to Float", &value.Int{V: 3, T: types.IntType}, types.FloatType, true, "*value.Float 3 0"},
		{"a computed integer to Float", &value.Int{V: 3, T: types.IntType}, types.FloatType, false, "*value.Int 3 1"},
		{"an Int8, its range left to storage", &value.Int{V: 300, T: types.IntType}, types.Int8Type, false, "*value.Int 300 0"},
		{"a string on Int", &value.Str{V: "x", T: types.StringType}, types.IntType, true, "*value.Str x 1"},
		{"a symbol on String", sym("x"), types.StringType, false, "*value.Symbol x 1"},
	}
	for _, c := range cases {
		v, fit := eval.ToBranch(c.v, c.t, c.written)
		if got := fmt.Sprintf("%T %s %d", v, text(v), fit); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

// text is a value's canonical text; a record's names its case and its fields as they stand.
func text(v value.Value) string {
	r, ok := v.(*value.Record)
	if !ok {
		return v.CanonText()
	}
	c := r.T.(*types.CaseType)
	if len(r.Fields) == 0 {
		return c.Name
	}
	return fmt.Sprintf("%s{%s: %v}", c.Name, c.Fields[0].Name, r.Fields[0])
}
