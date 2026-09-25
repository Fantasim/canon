package geninputsdata_test

import (
	"strings"
	"testing"

	p "example.com/data/geninputsdata/out/go"
	rt "example.com/data/geninputsdata/out/go/rt"
)

// TestInputs proves LoadInputs and its getters behave the same in data mode as in baked mode
// (CODEGEN.md §5.12): inputs are package state, independent of a value's mode.
func TestInputs(t *testing.T) {
	const want = "geninputsdata.Gen.required is a runtime input and LoadInputs has not been called"
	t.Run("panics with E8302 before LoadInputs", func(t *testing.T) {
		defer func() {
			r := recover()
			ee, ok := r.(*rt.EvalError)
			if !ok || ee.Code != "E8302" || ee.Message != want {
				t.Fatalf("recover = %#v, want *rt.EvalError{E8302, %q}", r, want)
			}
		}()
		(&p.Gen{}).Required()
		t.Fatal("want a panic")
	})

	t.Run("happy path, a refinement failure and a re-read", func(t *testing.T) {
		t.Setenv("GENINPUTSDATA_REQUIRED", "9")
		t.Setenv("GENINPUTSDATA_COUNT", "5")
		if err := p.LoadInputs(); err != nil {
			t.Fatalf("LoadInputs: %v", err)
		}
		if v, ok := (&p.Gen{}).Count(); !ok || v != 5 {
			t.Fatalf("Count() = %v, %v, want 5, true", v, ok)
		}

		t.Setenv("GENINPUTSDATA_COUNT", "1000")
		err := p.LoadInputs()
		if err == nil || !strings.Contains(err.Error(), "GENINPUTSDATA_COUNT: outside its refinement range") {
			t.Fatalf("err = %v, want it to say GENINPUTSDATA_COUNT: outside its refinement range", err)
		}
		if _, ok := (&p.Gen{}).Count(); ok {
			t.Error("Count: want unset after a failed re-read, not the stale value 5")
		}

		t.Setenv("GENINPUTSDATA_COUNT", "7")
		if err := p.LoadInputs(); err != nil {
			t.Fatalf("LoadInputs: %v", err)
		}
		if v, ok := (&p.Gen{}).Count(); !ok || v != 7 {
			t.Fatalf("Count() = %v, %v, want 7, true", v, ok)
		}
	})
}
