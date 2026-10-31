package geninputsdup_test

import (
	"strings"
	"testing"

	p "example.com/data/geninputsdup/out/go"
)

// TestEqualWires runs DECISIONS 383, 393 and EVALUATION.md §11.3: the first declared member holds a shared wire, and a retired first holder leaves its wire refused.
func TestEqualWires(t *testing.T) {
	for _, tc := range []struct {
		wire string
		want p.Mode
	}{{"x", p.ModeFirst}, {"y", p.ModeThird}} {
		t.Setenv("GENINPUTSDUP_MODE", tc.wire)
		if err := p.LoadInputs(); err != nil {
			t.Fatalf("LoadInputs(%q): %v", tc.wire, err)
		}
		if got, ok := (&p.Gen{}).Mode(); !ok || got != tc.want {
			t.Errorf("Mode() on %q = %v, %v, want %v", tc.wire, got, ok, tc.want)
		}
	}
	t.Setenv("GENINPUTSDUP_MODE", "z")
	err := p.LoadInputs()
	if err == nil || !strings.Contains(err.Error(), "GENINPUTSDUP_MODE: not a member of Mode") {
		t.Errorf("LoadInputs on the retired first holder's wire = %v, want a refusal", err)
	}
}
