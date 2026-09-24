package ir

import "testing"

// TestBelowRejectsAbove is E8007: a directory that only shares a root's path as a string
// prefix, but actually walks back out through "..", is not "under" that root.
func TestBelowRejectsAbove(t *testing.T) {
	cases := []struct {
		dir, root string
		wantRel   string
		wantOK    bool
	}{
		{"services/pipeline", "services", "pipeline", true},
		{"services", "services", "", true},
		{"services/../other", "services", "../other", false},
		{"..", ".", "..", false},
		{"../other", ".", "../other", false},
		{"other", ".", "other", true},
		{"../../../x", "../..", "../x", false},
		{"../../x", "../..", "x", true},
	}
	for _, c := range cases {
		rel, ok := below(c.dir, c.root)
		if rel != c.wantRel || ok != c.wantOK {
			t.Errorf("below(%q, %q) = %q, %v; want %q, %v", c.dir, c.root, rel, ok, c.wantRel, c.wantOK)
		}
	}
}
