package views_test

import (
	"slices"
	"testing"
)

// refTarget is a public record whose only link to a local record is a ref into a local table.
const refTarget = `package a

local record Thing {
  label: String
}

local let things: table Thing = {
  t1 { label: "One" }
}

record Holder {
  thing: ref things
}
`

// VIEWMODEL.md J12, I18N.md K1: a ref's target element is reachable, so the local Thing a
// public Holder refers to is in `types` (log-2026-09-28, i18n U6 review round 2).
func TestRefTargetReachable(t *testing.T) {
	m := demo(t, refTarget, "").model(t, demoPkg)
	names := make([]string, 0, len(m.Types))
	//canon:unordered the names are sorted below
	for name := range m.Types {
		names = append(names, name)
	}
	slices.Sort(names)
	if want := []string{"a.Holder", "a.Thing"}; !slices.Equal(names, want) {
		t.Errorf("types = %v, want %v", names, want)
	}
}
