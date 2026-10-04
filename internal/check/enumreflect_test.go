package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// STDLIB.md §3, TYPES.md §4.3, DECISIONS 299: E.members is [E] with no Selection; .retired is Bool.
func TestEnumMembersInfo(t *testing.T) {
	prog, f := checkSource(t, `package a

local enum E { a, retired b }

local let xs: [E] = E.members

local let r: Bool = E.a.retired
`)
	for _, s := range nodesOf[*syntax.SelectorExpr](f) {
		name := s.Name.Name
		obj := prog.Info.NameUses[s.Name]
		sel := prog.Info.Selections[s]
		switch name {
		case "members":
			if got := prog.Info.Types[s].String(); got != "[a.E]" {
				t.Errorf("Types[E.members] = %s, want [a.E]", got)
			}
			if obj == nil || obj.Kind() != check.ObjBuiltin || obj.Name() != name || sel != nil {
				t.Errorf("E.members names %v with selection %v, want the built-in members and none", obj, sel)
			}
		case "retired":
			if got := prog.Info.Types[s].String(); got != "Bool" {
				t.Errorf("Types[.retired] = %s, want Bool", got)
			}
			if sel == nil || sel.Kind != check.SelBuiltinMember {
				t.Errorf("Selections[.retired] = %v, want a built-in member", sel)
			}
		}
	}
}
