package edit_test

import (
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/edit"
)

// API.md V1, X2 (found by TestBenchEdit): a Duration set on a JSON field whose wire unit it is
// not a whole number of (`@json(unit: s)`, WIRE.md) is refused as a value, never an internal
// failure ("duration is a fraction of its field's unit").
func TestSetDurationFractionOfUnit(t *testing.T) {
	dir, roots := exampleRoots(t)
	fz := openFuzzProject(t, dir, roots)
	op := edit.Operation{Kind: edit.OpSet, Path: "resource.adventurequest:adventureQuests.global.completionRerollCooldown",
		Value: edit.Dur(214820 * time.Millisecond)}
	applyChecked(t, fz.env, fz.a, []edit.Operation{op}, false)
}
