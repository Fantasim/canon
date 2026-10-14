package eval

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
)

// TYPES.md §4.3: an index built from a partial member list is rebuilt; a vector's evaluator reads its root's.
func TestEnumIndexFollowsMembers(t *testing.T) {
	e := &types.EnumType{Name: "K", Members: []*types.Member{{Name: "x"}}}
	root := newEvaluator(check.Bags{}, Options{})
	if _, ok := root.enumIndex(e)["z"]; ok {
		t.Fatal("z found before it is a member")
	}
	e.Members = append(e.Members, &types.Member{Name: "y", Index: 1}, &types.Member{Name: "z", Index: 2})
	child := newEvaluator(check.Bags{}, Options{})
	child.parent = root
	if i, ok := child.enumIndex(e)["z"]; !ok || i != len(e.Members)-1 {
		t.Errorf("z = %d %t after completion, want %d", i, ok, len(e.Members)-1)
	}
	if len(root.enumNames[e]) != len(e.Members) {
		t.Errorf("the root keeps %d names, want %d", len(root.enumNames[e]), len(e.Members))
	}
}
