package edit_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// API.md V1, V2, X2 (found by FuzzMinimalWriteAll): a record in a JSON source set to a literal
// naming a dependent field's member by its bare word (DEP-02) is written or refused, never an
// internal failure ("value has no source wire here": the literal's Symbol has no wire form).
func TestSetJSONDependentSymbol(t *testing.T) {
	dir, roots := exampleRoots(t)
	fz := openFuzzProject(t, dir, roots)
	for _, op := range []edit.Operation{
		{Kind: edit.OpSet, Path: "resource.heistia:heistia.tasks[1].filterParam", Value: edit.Source("II_GEN_MAT_ORICHALCUM01")},
		{Kind: edit.OpSet, Path: "resource.heistia:heistia.tasks[1]", Value: edit.Source(
			`{ eventType: ECONOMY_DROP_ITEM, filterParam: II_GEN_MAT_MOONSTONE, targetPerPlayer: 10, maxDuration: 30m, description: "Drop moonstone" }`)},
	} {
		applyChecked(t, fz.env, fz.a, []edit.Operation{op}, false)
	}
}
