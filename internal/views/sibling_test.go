package views_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// VIEWMODEL.md J12: a per-instance target seen from a record its owner does not hold has no
// sibling, rather than a guessed `up`.
func TestSiblingNotReached(t *testing.T) {
	x := demo(t, sibling, "")
	node, tree := x.named(t, demoPkg, "Node"), x.named(t, demoPkg, "Tree")
	ref := shape.StripOptional(field(t, node, "parent").Type).Base().(*types.RefType)
	if s, perInstance := encode.Sibling(node, ref.Target); s == nil || !perInstance {
		t.Errorf("from Node: %v %v, want a sibling", s, perInstance)
	}
	if s, perInstance := encode.Sibling(field(t, tree, "nodes").Type, ref.Target); s != nil || !perInstance {
		t.Errorf("from a list type: %v %v, want none, per instance", s, perInstance)
	}
}
