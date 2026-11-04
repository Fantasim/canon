package types_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// keyed is a keyed list of package p named name whose key field is key, typed kt.
func keyed(name, key string, kt types.Type) (*types.Collection, *types.Field) {
	f := &types.Field{Name: key, Type: kt}
	return &types.Collection{Kind: types.CollLet, Pkg: "p", Name: name, KeyedBy: f}, f
}

// lazyLinks resolves a chain as the checker does: a ref named broken has no target, and a key
// field's type is read from types, not from the field.
type lazyLinks struct {
	broken *types.RefType
	types  map[*types.Field]types.Type
}

func (l lazyLinks) Target(r *types.RefType) *types.Collection {
	if r == l.broken {
		return nil
	}
	return r.Target
}

func (l lazyLinks) KeyType(f *types.Field) types.Type { return l.types[f] }

// WIRE.md §5.9, DECISIONS 456, TYPES.md §9.1: a chain ends at its leaf at every depth, at a table, a broken target or a loop.
func TestKeyChain(t *testing.T) {
	colour := &types.EnumType{Pkg: "p", Name: "Colour", Members: []*types.Member{{Name: "red"}}}
	tiers, id := keyed("tiers", "id", types.IntType)
	ranks, _ := keyed("ranks", "tier", &types.RefType{Target: tiers})
	grades, _ := keyed("grades", "rank", &types.RefType{Target: ranks})
	paints, _ := keyed("paints", "colour", colour)
	coats, _ := keyed("coats", "paint", &types.RefType{Target: paints})
	kinds := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "kinds"}
	aliases, kind := keyed("aliases", "kind", &types.RefType{Target: kinds})
	orphans, to := keyed("orphans", "to", &types.RefType{})
	loopA, toB := keyed("a", "b", nil)
	loopB, toA := keyed("b", "a", &types.RefType{Target: loopA})
	toB.Type = &types.RefType{Target: loopB}
	cases := []struct {
		name  string
		t     types.Type
		field *types.Field
		leaf  types.Type
		end   types.ChainEnd
	}{
		{"no ref", types.IntType, nil, types.IntType, types.ChainLeaf},
		{"depth 1", &types.RefType{Target: tiers}, id, types.IntType, types.ChainLeaf},
		{"depth 2", &types.RefType{Target: ranks}, id, types.IntType, types.ChainLeaf},
		{"depth 3", &types.RefType{Target: grades}, id, types.IntType, types.ChainLeaf},
		{"enum leaf", &types.RefType{Target: coats}, paints.KeyedBy, colour, types.ChainLeaf},
		{"table", &types.RefType{Target: kinds}, nil, types.StringType, types.ChainTable},
		{"table leaf", &types.RefType{Target: aliases}, kind, types.StringType, types.ChainTable},
		{"broken", &types.RefType{}, nil, types.ErrorType, types.ChainBroken},
		{"broken target", &types.RefType{Target: orphans}, to, types.ErrorType, types.ChainBroken},
		{"loop", &types.RefType{Target: loopA}, toA, nil, types.ChainLoop},
	}
	for _, c := range cases {
		got := types.KeyChain(c.t, types.Resolved)
		if got.Field != c.field || got.Leaf != c.leaf || got.End != c.end {
			t.Errorf("%s: KeyChain = %+v, want {%v %v %d}", c.name, got, c.field, c.leaf, c.end)
		}
	}
	if got := types.CollChain(grades, types.Resolved); got.Field != id || got.End != types.ChainLeaf {
		t.Errorf("CollChain(grades) = %+v, want the leaf id", got)
	}
	if got := types.CollChain(nil, types.Resolved); got.End != types.ChainBroken || got.Field != nil {
		t.Errorf("CollChain(nil) = %+v, want broken", got)
	}
}

// TYPES.md §10.2, DECISIONS 456: the walk reads every link through Links, as the checker resolves them.
func TestKeyChainLinks(t *testing.T) {
	tiers, id := keyed("tiers", "id", nil)
	broken := &types.RefType{Target: tiers}
	ranks, tier := keyed("ranks", "tier", nil)
	l := lazyLinks{broken: broken, types: map[*types.Field]types.Type{id: types.Int8Type, tier: &types.RefType{Target: tiers}}}
	if got := types.KeyChain(&types.RefType{Target: ranks}, l); got.Field != id || got.Leaf != types.Int8Type || got.End != types.ChainLeaf {
		t.Errorf("lazy chain = %+v, want the leaf id: Int8", got)
	}
	l.types[tier] = broken
	if got := types.KeyChain(&types.RefType{Target: ranks}, l); got.Field != tier || got.End != types.ChainBroken {
		t.Errorf("chain through a broken ref = %+v, want broken after tier", got)
	}
}
