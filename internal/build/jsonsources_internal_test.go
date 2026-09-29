package build

import (
	"testing"

	"github.com/fantasim/canonlang/internal/source"
)

// FORMATTER.md §14.1: two readings of one number that disagree keep it as written.
func TestReadFileConflict(t *testing.T) {
	rf := &readFile{content: []byte("[1.0, 2.50]"), texts: map[source.Span]string{}}
	one, two := source.Span{File: 3, Start: 1, End: 4}, source.Span{File: 4, Start: 6, End: 10}
	rf.note(one, "1")
	rf.note(one, "1.0000")
	rf.note(one, "1")
	rf.note(two, "2.5")
	rf.note(two, "2.5")
	got := rf.changed()
	if len(got) != 1 || got[0] != (Number{Start: 6, End: 10, Text: "2.5"}) {
		t.Errorf("changed = %+v", got)
	}
}
