package progen_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// WIRE.md §6.5: a link inside the project is followed; one outside or endless is skipped.
func TestLinks(t *testing.T) {
	c := examples(t)
	base := c.baselineOf([]string{"pipeline"})
	for _, tc := range []struct {
		target string
		warns  int
	}{
		{"II_POT_HEAL_S.json", 0},
		{"../data/II_POT_HEAL_L.json", 0},
		{"/p/pipeline/data/II_POT_HEAL_L.json", 0},
		{"/zzoutside/zz.json", 1},
		{"zzloop.json", 2},
	} {
		p := c.project.Clone()
		p.Link("pipeline/data/zzlink.json", tc.target)
		p.Link("pipeline/data/zzloop.json", "zzlink.json")
		if tc.target != "zzloop.json" {
			p.Link("pipeline/data/zzloop.json", "II_POT_HEAL_S.json")
		}
		out := progen.Run(context.Background(), p, progen.RunOptions{Packages: []string{"pipeline"}, Roots: exampleRoots()})
		warns := 0
		for _, f := range out.Findings {
			switch {
			case base[keyOf(f)]: // pipeline's own baseline, not a link finding
			case f.Code == diag.W7115.Def().Code:
				warns++
			default:
				t.Errorf("%s: unexpected %v %q", tc.target, f, f.Message)
			}
		}
		if out.Err != nil || warns != tc.warns {
			t.Errorf("%s: %d links skipped (want %d), error %v", tc.target, warns, tc.warns, out.Err)
		}
	}
}
