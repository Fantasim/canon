package edit

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// DECISIONS 316, ERRORS.md E3012: a ref key that leads back to a list already passed (a list keyed
// by a ref to itself, or two keyed by refs to each other) names no key; refWant ends.
func TestRefWantCycle(t *testing.T) {
	self := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "ns"}
	self.KeyedBy = &types.Field{Name: "p", Type: &types.RefType{Target: self}}
	a := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "as"}
	b := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "bs", KeyedBy: &types.Field{Name: "a", Type: &types.RefType{Target: a}}}
	a.KeyedBy = &types.Field{Name: "b", Type: &types.RefType{Target: b}}
	for _, coll := range []*types.Collection{self, a} {
		if _, ok := refWant(KeyLit{Kind: KeyWord, Text: "x", Raw: "x"}, coll); ok {
			t.Errorf("DECISIONS 316: refWant through %s found a key", coll.Name)
		}
	}
	base := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "ms"}
	via := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "sp", KeyedBy: &types.Field{Name: "m", Type: &types.RefType{Target: base}}}
	if k, ok := refWant(KeyLit{Kind: KeyWord, Text: "wolf", Raw: "wolf"}, via); !ok || k.S != "wolf" {
		t.Errorf("API.md P1: refWant through sp to ms: %+v %v", k, ok)
	}
}
