package edit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/edit"
)

// API.md V1, X2; WIRE.md 5.1 (log-2026-09-29 M4 U7b-r2, B9): a Duration a JSON source cannot
// write as a whole number of its field's unit (E8102) is a *ValueError before anything is
// written, never an internal failure; a whole number of the unit is written.
func TestSetDurationFractionOfUnit(t *testing.T) {
	dir, roots := exampleRoots(t)
	fz := openFuzzProject(t, dir, roots)
	for _, c := range []struct {
		name, path string
		v          edit.Lit
		refused    bool
	}{
		{"s, 214820 ms", "resource.adventurequest:adventureQuests.global.completionRerollCooldown", edit.Dur(214820 * time.Millisecond), true},
		{"s, 3 ms", "resource.heistia:heistia.buffPool[#0].duration", edit.Dur(3 * time.Millisecond), true},
		{"m, -149 ms", "resource.heistia:heistia.tasks[#2].maxDuration", edit.Dur(-149 * time.Millisecond), true},
		{"s, Source", "resource.heistia:heistia.buffPool[#0].duration", edit.Source("1500ms"), true},
		{"s, whole", "resource.heistia:heistia.buffPool[#0].duration", edit.Dur(7 * time.Second), false},
		{"m, whole", "resource.heistia:heistia.tasks[#2].maxDuration", edit.Source("2m"), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			op := edit.Operation{Kind: edit.OpSet, Path: c.path, Value: c.v}
			_, err := edit.Apply(context.Background(), fz.env, edit.NewSnapshot(fz.a), edit.Request{Ops: []edit.Operation{op}})
			var ve *edit.ValueError
			switch {
			case errors.Is(err, edit.ErrInternal):
				t.Fatalf("Apply = %v, an internal failure", err)
			case c.refused && !errors.As(err, &ve):
				t.Fatalf("Apply = %v, want a *ValueError", err)
			case !c.refused && err != nil:
				t.Fatalf("Apply = %v, want the value written", err)
			}
		})
	}
}
