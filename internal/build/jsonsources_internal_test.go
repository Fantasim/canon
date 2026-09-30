package build

import (
	"testing"

	"github.com/fantasim/canonlang/internal/source"
)

// FORMATTER.md 14.1 (log-2026-09-29 M4 B11-r3): two readings of one number that disagree on its
// text or on its base type keep it as written; readings that agree on both make it canonical.
func TestReadFileConflict(t *testing.T) {
	rf := newReadFile([]byte("[1.0, 2.50, 3.0]"))
	one := source.Span{File: 3, Start: 1, End: 4}
	two := source.Span{File: 4, Start: 6, End: 10}
	three := source.Span{File: 4, Start: 12, End: 15}
	rf.note(one, "1", "Float")
	rf.note(one, "1.0000", "Float")
	rf.note(one, "1", "Float")
	rf.note(two, "2.5", "Float")
	rf.note(two, "2.5", "Float")
	rf.note(three, "3", "Float")
	rf.note(three, "3", "Float32")
	got := rf.changed()
	if len(got) != 1 || got[0] != (Number{Start: 6, End: 10, Text: "2.5"}) {
		t.Errorf("changed = %+v", got)
	}
}
