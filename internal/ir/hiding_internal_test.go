package ir

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// hideExempt is the item-less names of the namespace and of detail that hideDeclared leaves out of E8005 although they look like an access member's (Load<V>, Resolve): names the generator fixes, which no decoder body names unqualified (CODEGEN.md §3.5, exemption (c) of hideDeclared).
var hideExempt = []string{loadInputs}

// CODEGEN.md §2.11, §3.5, DECISIONS 435: hideDeclared skips a declared name that has no item, which is sound only while every such name in the namespace and in detail is one no access member can be (not Load<V>, LoadSnapshot or Resolve) but for the exempt set; this pins the set on one plan with inputs, and TestFindings and TestCascades pin it on the plans of every case (CppUnexemptLoads), so a new item-less Load... or Resolve name fails instead of passing unreported. A value named like the exempt LoadInputs is no collision.
func TestHideDeclaredExemptsOnlyTheFixedNames(t *testing.T) {
	rec := &Record{Name: "Settings", Pkg: "a", Fields: []*Field{{Name: "key", Type: TypeRef{Kind: types.String}, Input: &types.Input{Env: "KEY"}}}}
	e := &Emit{Target: TargetCpp, Mode: ModeData, Namespace: "a"}
	p := &Package{Name: "a", Types: []Type{rec}, Emits: []*Emit{e}, Values: []*Value{{Name: "inputs", Type: TypeRef{Kind: types.Record, Named: rec}}}}
	pl := PlanCppNames(p, e)
	if got := pl.Problems(); len(got) != 0 {
		t.Fatalf("problems = %+v, want none: a value named inputs and LoadInputs meet legally", got)
	}
	if got := cppItemlessLoads(pl); !slices.Equal(got, hideExempt) {
		t.Errorf("item-less Load.../Resolve names of the namespace and detail = %v, want the exempt set %v", got, hideExempt)
	}
}

// cppItemlessLoads are the names of pl's namespace and detail declared for no item that an access member could be named (Load<V>, LoadSnapshot, Resolve), sorted.
func cppItemlessLoads(pl *CppNamePlan) []string {
	var out []string
	for _, sc := range pl.scopes {
		if sc != pl.ns && sc.what != cppDetail {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(sc.names)) {
			if sc.items[name] == nil && (strings.HasPrefix(name, CppLoad) || name == CppResolve) {
				out = append(out, name)
			}
		}
	}
	slices.Sort(out)
	return out
}
