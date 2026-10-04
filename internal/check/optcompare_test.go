package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// TYPES.md §7.5 (TYP-08), §4.1: a ref C? name compared with a ref C is the name, never a key.
func TestOptionalRefOperandIsTheName(t *testing.T) {
	prog, f := checkSource(t, `package a

local record C { name: String }

local record O {
  cols: [C] keyed by name
  pick: ref C? = none
  same: ref C
  check same == pick and pick == same and same != pick and pick != same and same != zed else "x"
}
`)
	names := map[string]int{}
	for _, id := range nodesOf[*syntax.IdentExpr](f) {
		names[id.Name]++
		o := prog.Info.Uses[id]
		switch id.Name {
		case "pick", "same":
			if prog.Info.Keys[id] != nil || o == nil || o.Kind() != check.ObjField {
				t.Errorf("%s: Keys %v, Uses %v, want the field", id.Name, prog.Info.Keys[id], o)
			}
		case "zed":
			if coll := prog.Info.Keys[id]; coll == nil {
				t.Errorf("zed: Keys %v, want a key of cols", coll)
			}
		}
	}
	if names["pick"] != 4 || names["same"] != 5 || names["zed"] != 1 {
		t.Errorf("names seen %v", names)
	}
}
