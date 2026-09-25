package geninputsbaked_test

import (
	"strings"
	"testing"
	"time"

	p "example.com/data/geninputsbaked/out/go"
	rt "example.com/data/geninputsbaked/out/go/rt"
)

const e8302Want = "geninputsbaked.Gen.required is a runtime input and LoadInputs has not been called"

// TestInputs runs CODEGEN.md §5.12 and EVALUATION.md §11.3's rules, in order: the first subtest
// needs LoadInputs to never have run yet, so every other subtest follows it.
func TestInputs(t *testing.T) {
	t.Run("panics with E8302 before LoadInputs", func(t *testing.T) {
		defer func() {
			r := recover()
			ee, ok := r.(*rt.EvalError)
			if !ok || ee.Code != "E8302" || ee.Message != e8302Want {
				t.Fatalf("recover = %#v, want *rt.EvalError{E8302, %q}", r, e8302Want)
			}
		}()
		(&p.Gen{}).Required()
		t.Fatal("want a panic")
	})

	t.Run("happy path", func(t *testing.T) {
		t.Setenv("GENINPUTSBAKED_FLAG", "true")
		t.Setenv("GENINPUTSBAKED_COUNT", "5")
		t.Setenv("GENINPUTSBAKED_RATIO", "0.5")
		t.Setenv("GENINPUTSBAKED_SPAN", "30s")
		t.Setenv("GENINPUTSBAKED_NAME", "hello")
		t.Setenv("GENINPUTSBAKED_CODE", "OK")
		t.Setenv("GENINPUTSBAKED_SCALE", "2.5")
		t.Setenv("GENINPUTSBAKED_MODE", "fast")
		t.Setenv("GENINPUTSBAKED_REQUIRED", "9")
		if err := p.LoadInputs(); err != nil {
			t.Fatalf("LoadInputs: %v", err)
		}
		g := &p.Gen{}
		if v, ok := g.Flag(); !ok || !v {
			t.Errorf("Flag() = %v, %v", v, ok)
		}
		if v, ok := g.Count(); !ok || v != 5 {
			t.Errorf("Count() = %v, %v", v, ok)
		}
		if v, ok := g.Ratio(); !ok || v != 0.5 {
			t.Errorf("Ratio() = %v, %v", v, ok)
		}
		if v, ok := g.Span(); !ok || v != 30*time.Second {
			t.Errorf("Span() = %v, %v", v, ok)
		}
		if v, ok := g.Name(); !ok || v != "hello" {
			t.Errorf("Name() = %q, %v", v, ok)
		}
		if v, ok := g.Code(); !ok || v != "OK" {
			t.Errorf("Code() = %q, %v", v, ok)
		}
		if v, ok := g.Scale(); !ok || v != 2.5 {
			t.Errorf("Scale() = %v, %v", v, ok)
		}
		if v, ok := g.Mode(); !ok || v != p.ModeFast {
			t.Errorf("Mode() = %v, %v", v, ok)
		}
		if v := g.Required(); v != 9 {
			t.Errorf("Required() = %v", v)
		}
	})

	t.Run("every parse error kind", func(t *testing.T) {
		t.Setenv("GENINPUTSBAKED_REQUIRED", "1")
		t.Setenv("GENINPUTSBAKED_FLAG", "maybe")
		t.Setenv("GENINPUTSBAKED_COUNT", "x")
		t.Setenv("GENINPUTSBAKED_RATIO", "x")
		t.Setenv("GENINPUTSBAKED_SPAN", "x")
		t.Setenv("GENINPUTSBAKED_NAME", "x")
		t.Setenv("GENINPUTSBAKED_SCALE", "x")
		t.Setenv("GENINPUTSBAKED_MODE", "unknown")
		err := p.LoadInputs()
		if err == nil {
			t.Fatal("want an error")
		}
		for _, want := range []string{
			"GENINPUTSBAKED_FLAG: not a valid Bool", "GENINPUTSBAKED_COUNT: not a valid Int",
			"GENINPUTSBAKED_RATIO: not a valid Float", "GENINPUTSBAKED_SPAN: not a valid Duration",
			"GENINPUTSBAKED_SCALE: not a valid Float", "GENINPUTSBAKED_MODE: not a member of Mode",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %v: missing %q", err, want)
			}
		}
	})

	t.Run("refinement failures, a Float32 overflow and a retired member", func(t *testing.T) {
		t.Setenv("GENINPUTSBAKED_REQUIRED", "1")
		t.Setenv("GENINPUTSBAKED_FLAG", "")
		t.Setenv("GENINPUTSBAKED_COUNT", "1000")
		t.Setenv("GENINPUTSBAKED_RATIO", "2")
		t.Setenv("GENINPUTSBAKED_SPAN", "2m")
		t.Setenv("GENINPUTSBAKED_NAME", "toolong")
		t.Setenv("GENINPUTSBAKED_CODE", "not-upper")
		t.Setenv("GENINPUTSBAKED_SCALE", "1e40")
		t.Setenv("GENINPUTSBAKED_MODE", "old")
		err := p.LoadInputs()
		if err == nil {
			t.Fatal("want an error")
		}
		for _, want := range []string{
			"GENINPUTSBAKED_COUNT: outside its refinement range", "GENINPUTSBAKED_RATIO: outside its refinement range",
			"GENINPUTSBAKED_SPAN: outside its refinement range", "GENINPUTSBAKED_NAME: outside its refinement range",
			"GENINPUTSBAKED_CODE: does not match its pattern", "GENINPUTSBAKED_SCALE: outside its refinement range",
			"GENINPUTSBAKED_MODE: not a member of Mode",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %v: missing %q", err, want)
			}
		}
	})

	t.Run("a Float32 range check runs on the rounded value", func(t *testing.T) {
		t.Setenv("GENINPUTSBAKED_REQUIRED", "1")
		t.Setenv("GENINPUTSBAKED_SCALE", "10.0000001")
		if err := p.LoadInputs(); err != nil {
			t.Fatalf("LoadInputs: %v", err)
		}
		if v, ok := (&p.Gen{}).Scale(); !ok || v != 10 {
			t.Fatalf("Scale() = %v, %v, want 10, true (rounds to the range's edge)", v, ok)
		}
	})

	t.Run("an unset or empty variable is unset", func(t *testing.T) {
		t.Setenv("GENINPUTSBAKED_REQUIRED", "1")
		t.Setenv("GENINPUTSBAKED_FLAG", "")
		t.Setenv("GENINPUTSBAKED_COUNT", "")
		t.Setenv("GENINPUTSBAKED_RATIO", "")
		t.Setenv("GENINPUTSBAKED_SPAN", "")
		t.Setenv("GENINPUTSBAKED_NAME", "")
		t.Setenv("GENINPUTSBAKED_CODE", "")
		t.Setenv("GENINPUTSBAKED_SCALE", "")
		t.Setenv("GENINPUTSBAKED_MODE", "")
		if err := p.LoadInputs(); err != nil {
			t.Fatalf("LoadInputs: %v", err)
		}
		g := &p.Gen{}
		if _, ok := g.Flag(); ok {
			t.Error("Flag: want unset")
		}
		if _, ok := g.Count(); ok {
			t.Error("Count: want unset")
		}
		if _, ok := g.Scale(); ok {
			t.Error("Scale: want unset")
		}
		if _, ok := g.Mode(); ok {
			t.Error("Mode: want unset")
		}
		if v := g.Required(); v != 1 {
			t.Errorf("Required() = %v, want 1", v)
		}
	})

	t.Run("a required variable missing is an error naming it", func(t *testing.T) {
		t.Setenv("GENINPUTSBAKED_REQUIRED", "")
		err := p.LoadInputs()
		if err == nil || !strings.Contains(err.Error(), "GENINPUTSBAKED_REQUIRED: not set") {
			t.Errorf("err = %v, want it to say GENINPUTSBAKED_REQUIRED: not set", err)
		}
	})

	t.Run("LoadInputs re-reads the environment", func(t *testing.T) {
		t.Setenv("GENINPUTSBAKED_REQUIRED", "1")
		t.Setenv("GENINPUTSBAKED_COUNT", "1")
		if err := p.LoadInputs(); err != nil {
			t.Fatalf("LoadInputs: %v", err)
		}
		if v, ok := (&p.Gen{}).Count(); !ok || v != 1 {
			t.Fatalf("Count() = %v, %v, want 1, true", v, ok)
		}
		t.Setenv("GENINPUTSBAKED_COUNT", "2")
		if err := p.LoadInputs(); err != nil {
			t.Fatalf("LoadInputs: %v", err)
		}
		if v, ok := (&p.Gen{}).Count(); !ok || v != 2 {
			t.Fatalf("Count() = %v, %v, want 2, true", v, ok)
		}
	})

	t.Run("a re-read that fails leaves the field unset, not stale", func(t *testing.T) {
		t.Setenv("GENINPUTSBAKED_REQUIRED", "1")
		t.Setenv("GENINPUTSBAKED_COUNT", "3")
		if err := p.LoadInputs(); err != nil {
			t.Fatalf("LoadInputs: %v", err)
		}
		if v, ok := (&p.Gen{}).Count(); !ok || v != 3 {
			t.Fatalf("Count() = %v, %v, want 3, true", v, ok)
		}
		t.Setenv("GENINPUTSBAKED_COUNT", "not-an-int")
		if err := p.LoadInputs(); err == nil {
			t.Fatal("want an error")
		}
		if _, ok := (&p.Gen{}).Count(); ok {
			t.Error("Count: want unset after a failed re-read, not the stale value 3")
		}
	})
}
