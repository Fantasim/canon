package load

import (
	"context"
	"math/big"
	"testing"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// DECISIONS 173: wireHost.Default reads a default literally, redundant parentheses unwrapped.
func TestWireHostDefault(t *testing.T) {
	via := &value.Prov{}
	optInt := &types.OptionalType{Elem: types.IntType}
	cases := []struct {
		name string
		f    *types.Field
		want string
	}{
		{"none", &types.Field{Default: &syntax.NoneLit{}}, "none"},
		{"bool", &types.Field{Default: &syntax.BoolLit{Value: true}}, "true"},
		{"int", &types.Field{Type: types.IntType, Default: &syntax.IntLit{Value: big.NewInt(7)}}, "7"},
		{"duration", &types.Field{Default: &syntax.DurationLit{Millis: 1500}}, "1s500ms"},
		{"string", &types.Field{Type: types.StringType, Default: str(syntax.StringPart{Text: "x"})}, "x"},
		{"optional with no default", &types.Field{Type: optInt}, "none"},
		{"parenthesized string", &types.Field{Type: types.StringType, Default: &syntax.ParenExpr{X: str(syntax.StringPart{Text: "szName"})}}, "szName"},
		{"doubly parenthesized int", &types.Field{Type: types.IntType, Default: &syntax.ParenExpr{X: &syntax.ParenExpr{X: &syntax.IntLit{Value: big.NewInt(7)}}}}, "7"},
		{"parenthesized none", &types.Field{Type: optInt, Default: &syntax.ParenExpr{X: &syntax.NoneLit{}}}, "none"},
	}
	for _, c := range cases {
		v, ok := wireHost{}.Default(context.Background(), c.f, wire.Instance{}, via)
		if !ok {
			t.Errorf("%s: not ok", c.name)
			continue
		}
		if got := v.CanonText(); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
		if p := v.Prov(); p == nil || p.Kind != value.ProvDefault || p.Via != via {
			t.Errorf("%s: provenance %+v", c.name, p)
		}
	}
}

// meta/decisions log "M3 W1 eval … calls": an omitted optional field's provenance is default,
// span the load call's own (not zero), via the object that omitted it.
func TestWireHostDefaultNoneSpan(t *testing.T) {
	span := source.Span{Start: 3, End: 5}
	via := &value.Prov{Kind: value.ProvJSON}
	f := &types.Field{Type: &types.OptionalType{Elem: types.IntType}}
	v, ok := wireHost{span: span}.Default(context.Background(), f, wire.Instance{}, via)
	if !ok {
		t.Fatal("not ok")
	}
	n, isNone := v.(*value.None)
	p := v.Prov()
	if !isNone || p == nil || p.Kind != value.ProvDefault || p.Span != span || p.Via != via {
		t.Errorf("none = %+v, prov = %+v, want span %+v via %+v", n, p, span, via)
	}
}

// wireHost.Deref is never reached this milestone: supported refuses a decode with a ref field.
func TestWireHostDeref(t *testing.T) {
	if r, ok := (wireHost{}).Deref(context.Background(), &value.Ref{}); ok || r != nil {
		t.Errorf("Deref = %v, %v, want nil, false", r, ok)
	}
}
