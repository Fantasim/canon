package edit

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
)

// broadList is a `[ ]` list broken over lines by its width alone.
var broadList = "[\n  \"" + strings.Repeat("t", 36) + "\",\n  \"" + strings.Repeat("u", 36) + "\",\n  \"" + strings.Repeat("w", 36) + "\",\n]"

// API.md M5, M6: a list narrowed by a Replace is predicted to join; the same Replace with an
// Insert that widens it again, or beside a Replace whose text spans lines, is not, as the
// formatter keeps the list broken (review 2026-10-13 NIT 4).
func TestJoinCountsEveryChange(t *testing.T) {
	e, f := parseLiteral(broadList)
	l, ok := e.(*syntax.ListLit)
	if !ok {
		t.Fatalf("not a list: %v", e)
	}
	tr := newCanonTree(f)
	narrow := format.Change{Kind: format.Replace, Node: l.Elems[0], Text: `"x"`}
	grow := format.Change{Kind: format.Insert, List: l.First(), At: len(l.Elems), Text: `"` + strings.Repeat("y", 40) + `"`}
	lines := format.Change{Kind: format.Replace, Node: l.Elems[1], Text: "\"\"\"\n  a\n  \"\"\""}
	cases := []struct {
		name    string
		changes []format.Change
		joins   bool
	}{
		{"narrowed", []format.Change{narrow}, true},
		{"narrowed, then widened", []format.Change{narrow, grow}, false},
		{"beside a text of lines", []format.Change{narrow, lines}, false},
	}
	for _, c := range cases {
		if got := len(tr.joins(c.changes)) > 0; got != c.joins {
			t.Errorf("%s: joins %v, want %v", c.name, got, c.joins)
		}
	}
}

// nestedList is a broken `[ ]` list holding a string of n letters and the list inner.
func nestedList(n int, inner string) string {
	return "[\n  \"" + strings.Repeat("a", n) + "\",\n  " + inner + ",\n]"
}

// API.md M5, M6: an Insert into an empty `[ ]` list adds no separator, so the list it fills to
// the formatter's width exactly joins; a Remove emptying one takes none away, so a list one past
// the width after it does not (review 2026-10-13 NITs 2, 3).
func TestJoinCountsEmptyLists(t *testing.T) {
	e, f := parseLiteral(nestedList(80, "[]"))
	l := e.(*syntax.ListLit)
	filled := []format.Change{
		{Kind: format.Replace, Node: l.Elems[0], Text: `"x"`},
		{Kind: format.Insert, List: l.Elems[1].First(), Text: `"` + strings.Repeat("y", 80) + `"`},
	}
	if len(newCanonTree(f).joins(filled)) == 0 {
		t.Error("a list filled to the width exactly: no join, want one")
	}
	e, f = parseLiteral(nestedList(84, `["z"]`))
	l = e.(*syntax.ListLit)
	inner := l.Elems[1].(*syntax.ListLit)
	emptied := []format.Change{{Kind: format.Remove, Node: inner.Elems[0]}}
	if len(newCanonTree(f).joins(emptied)) != 0 {
		t.Error("a list one past the width once emptied: a join, want none")
	}
}
