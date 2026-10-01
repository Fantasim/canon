package ir

import (
	"testing"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

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
