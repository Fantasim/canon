package build

import (
	"context"
	"testing"
)

const (
	placedProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    data: \"../A\"\n  }\n  optional_roots: [data]\n}\n"
	placedSource  = "/// A.\npackage a\n\n/// N, which must be 1.\nlet n: Int(1..=1) = load(\"@data/n.json\")\n"
)

// placedCase is a project whose optional root data is A, holding n = 2, and whose package checks
// n == 1.
func placedCase() roFS {
	return roFS{
		"law/project.canon": srcFile(placedProject), "law/a/a.canon": srcFile(placedSource),
		"A/n.json": srcFile("2\n"), "B/n.json": srcFile("1\n"),
	}
}

// DECISIONS 332: where this machine places the roots, and which are absent, are part of the
// check-cache key, and a persistent Cache never serves a result read under another placement.
func TestPlacedKey(t *testing.T) {
	ctx := context.Background()
	fsys := placedCase()
	p, err := Open(fsys, "/law", Options{})
	if err != nil {
		t.Fatal(err)
	}
	p = p.WithCache(NewCache())
	steps := []struct {
		name   string
		do     func()
		errors int
	}{
		{"declared", func() {}, 1},
		{"moved by project.local.canon", func() {
			fsys["law/project.local.canon"] = srcFile("project acme {\n  roots {\n    data: \"../B\"\n  }\n}\n")
		}, 0},
		{"moved to an absent directory", func() {
			fsys["law/project.local.canon"] = srcFile("project acme {\n  roots {\n    data: \"../C\"\n  }\n}\n")
		}, 1},
		{"the directory appears", func() { fsys["C/n.json"] = srcFile("1\n") }, 0},
	}
	var keys [][32]byte
	for _, st := range steps {
		st.do()
		s, err := p.open()
		if err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
		keys = append(keys, s.canon)
		a, err := p.Analyze(ctx, nil)
		if err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
		if got := a.Result().Summary.Errors; got != st.errors {
			t.Errorf("%s: %d errors, want %d: %v", st.name, got, st.errors, a.Result().List)
		}
	}
	for i := 1; i < len(keys); i++ {
		if keys[i] == keys[i-1] {
			t.Errorf("%s: the check key did not change", steps[i].name)
		}
	}
}
