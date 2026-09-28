package ir

import "testing"

// CODEGEN.md §3.5, decision 203: a self-meeting item is reported against the package, apart from its meeting a package name; a cause met twice is one E8005.
func TestCollideSelfPair(t *testing.T) {
	n := newNamer("a", cppValidIdent)
	sc := &nameScope{what: "a.Things"}
	n.collide(sc, "Things", "a.things", "a.things", nil)
	n.collide(sc, "LoadInputs", "a.things", "a", nil)
	n.collide(sc, "Find", "a.things", "a.things", nil)
	if len(n.problems) != 2 {
		t.Fatalf("problems = %+v, want 2", n.problems)
	}
	for i, want := range []string{"Things", "LoadInputs"} {
		if p := n.problems[i]; p.Name != want || p.First != "a.things" || p.Origin != "a" {
			t.Errorf("problem %d = %+v, want %s between a.things and a", i, p, want)
		}
	}
}
