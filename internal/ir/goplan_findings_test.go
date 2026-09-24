package ir_test

import (
	"math"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// mathDep is a package whose go emit is named math, and an importer that names its type and holds a list of Floats.
const (
	mathDep = `package b

/// A colour.
enum Tone { red, blue }

emit go { out: "@features/b", package: "math" }
`
	mathUser = `package a

import b { Tone }

/// A badge.
record Badge {
  /// Its colour.
  tone: Tone
}

/// The weights.
const weights = [1.5]

emit go { out: "@features/a", package: "a" }
`
)

// TestGoImportCollidesOnlyWhenWritten is DECISIONS 202 and CODEGEN.md §2.8, §3.5: baked Go imports math only for a -0.0 literal, here a list constant's element (a scalar -0.0 constant is refused, decision 181), so a Canon import whose go emit is named math collides with it (E8005, at the emit) only when the package writes one.
func TestGoImportCollidesOnlyWhenWritten(t *testing.T) {
	e8005 := "[" + string(diag.E8005.Def().Code) + "]"
	for _, c := range []struct {
		weight float64
		want   bool
	}{{1.5, false}, {math.Copysign(0, -1), true}} {
		w := newWorld(t)
		w.add(t, "b/b.canon", []byte(mathDep))
		w.add(t, "a/a.canon", []byte(mathUser))
		w.check(t)
		weight := &value.Float{V: c.weight, T: types.FloatType}
		w.cache["a.weights"] = &value.List{T: &types.ListType{Elem: types.FloatType}, Elems: []value.Value{weight}} // WIRE.md §5.1 reads -0.0 as 0: only evaluation makes one
		w.calls = w.fixtureCalls
		w.build(t)
		out := w.findings(t)
		if got := strings.Contains(out, e8005) && strings.Contains(out, "math"); got != c.want {
			t.Errorf("weight %v: %s on math %v, want %v:\n%s", c.weight, e8005, got, c.want, out)
		}
	}
}

// TestGoTypesModeHasNoPlan is DECISIONS 203: a go emit in a mode gen/go has no generator for (types, embedded) is checked only for its overrides, so two enum members meeting in ToneSeries1 are not E8005 there; data mode has its plan since M2 (testdata/findings/E8005_6.txtar).
func TestGoTypesModeHasNoPlan(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(`package a

/// A tone.
enum Tone { series_1, series1 }

emit go { out: "@features/a", package: "a", mode: types }
`))
	w.calls = w.fixtureCalls
	w.build(t)
	if out := w.findings(t); strings.Contains(out, "["+string(diag.E8005.Def().Code)+"]") {
		t.Errorf("a types-mode go emit has no name plan yet:\n%s", out)
	}
}
