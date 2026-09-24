package load

import (
	"context"
	"math/big"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// EVALUATION.md §13's Prov (DECISIONS 173): wireHost.Default reads a field's default literally.
func TestWireHostDefault(t *testing.T) {
	via := &value.Prov{}
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

// wireHost.Deref is never reached this milestone: supported refuses a decode with a ref field.
func TestWireHostDeref(t *testing.T) {
	if r, ok := (wireHost{}).Deref(context.Background(), &value.Ref{}); ok || r != nil {
		t.Errorf("Deref = %v, %v, want nil, false", r, ok)
	}
}
