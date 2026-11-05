package check_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// amendKeyBase holds a keyed list two levels down, and a name in scope.
const amendKeyBase = `package a

local record S {
  k: String
  w: Int = 0
}

local record A {
  id: String
  oo: [S] keyed by k
}

local record Bag {
  aa: [A] keyed by id
}

local let aa: [A] keyed by id = [{ id: "second", oo: [{ k: "x" }] }]

local let b: Bag = { aa: [{ id: "second", oo: [{ k: "x" }] }] }

local let pick: String = "x"
`

// amendKeyLayer amends an element of a keyed list after an entry step by a bare name, and by a name in scope.
const amendKeyLayer = `package a
layer dev

amend aa {
  second.oo[x].w: 5
  second.oo[pick].w: 6
}

amend b {
  aa[second].oo[x].w: 7
}
`

// EVALUATION.md §9.2, TYPES.md §4.1: `[x]` after an entry step is a key recorded in Keys; a name in scope stays an expression.
func TestAmendBracketKeyAfterEntry(t *testing.T) {
	prog, files, out := checkSet(t, [][2]string{{"a/a.canon", amendKeyBase}, {"a/dev.layer.canon", amendKeyLayer}})
	if !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	for _, f := range files {
		for _, g := range gaps(f, prog.Info, func(syntax.Decl) bool { return false }) {
			t.Error(g)
		}
	}
	layer := files[1]
	for _, tc := range []struct {
		name      string
		line, col int
		isKey     bool
	}{
		{"after `.second` of the let", 5, 13, true},
		{"after `[second]` of a record field's list", 10, 17, true},
		{"a name in scope is an expression (DECISIONS 318)", 6, 13, false},
	} {
		var id *syntax.IdentExpr
		for _, e := range nodesOf[*syntax.IdentExpr](layer) {
			if l, c := layer.Src.Position(layer.Span(e).Start); l == tc.line && c == tc.col {
				id = e
			}
		}
		if id == nil {
			t.Fatalf("%s: no name at %d:%d", tc.name, tc.line, tc.col)
		}
		coll := prog.Info.Keys[id]
		if tc.isKey && !isFieldColl(coll, "oo") {
			t.Errorf("%s: Keys[%s] = %v, want the collection of field oo", tc.name, id.Name, coll)
		}
		if !tc.isKey && (coll != nil || prog.Info.Uses[id] == nil) {
			t.Errorf("%s: %s is a key (%v) or resolves to nothing", tc.name, id.Name, coll)
		}
	}
}

// isFieldColl reports a collection held by the record field named field.
func isFieldColl(c *types.Collection, field string) bool {
	return c != nil && c.Kind == types.CollField && len(c.FieldPath) == 1 && c.FieldPath[0] == field
}
