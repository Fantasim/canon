package ir

import (
	"testing"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

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

// TestFileDir is WIRE.md §2.2: an unrooted path starts at its file's directory, "" for a file at the project root, as check has it.
func TestFileDir(t *testing.T) {
	for _, c := range []struct{ file, want string }{
		{"a.canon", ""},
		{"a/a.canon", "a"},
		{"a/b/c.canon", "a/b"},
	} {
		f := &syntax.File{Src: &source.File{Path: c.file}}
		if got := fileDir(f); got != c.want {
			t.Errorf("fileDir(%q) = %q, want %q", c.file, got, c.want)
		}
	}
	if got := fileDir(&syntax.File{}); got != "" {
		t.Errorf("fileDir without a source = %q, want none", got)
	}
}
