package edit

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
)

// joins are what the settle lays on one line again as changes narrow it (API.md M5, M6): a value
// alone on the line after its `:` or `=` that now fits beside them, and each `[ ]` list, below
// any broken brace list, broken by width alone that now fits, with the break before it.
func (t *canonTree) joins(changes []format.Change) []region {
	var out []region
	seen := map[syntax.Node]bool{}
	for _, c := range changes {
		if !narrowing(c) {
			continue
		}
		if c.Kind == format.Replace {
			out = append(out, t.valueJoin(c, changes)...)
			out = append(out, t.keyJoin(c, changes)...)
		}
		for _, l := range t.listsAbove(c.Node) {
			if !seen[l] {
				seen[l] = true
				out = append(out, t.listJoin(l, changes)...)
			}
		}
	}
	return out
}

// listsAbove are the `[ ]` lists holding n, nearest first, up to a brace list broken over lines.
func (t *canonTree) listsAbove(n syntax.Node) []syntax.Node {
	var out []syntax.Node
	for up := t.parent[n]; up != nil && up != syntax.Node(t.f); up = t.parent[up] {
		if _, brace := up.(*syntax.BraceLit); brace && !t.oneLine(up) {
			break
		}
		if _, list := up.(*syntax.ListLit); list {
			out = append(out, up)
		}
	}
	return out
}

// valueJoin is the break before the node c prints again when that node sat on a line of its own
// after its `:` or `=` and, narrower, now fits beside them.
func (t *canonTree) valueJoin(c format.Change, changes []format.Change) []region {
	n := t.climb(c.Node)
	sep, broken := t.breakBefore(n)
	if !broken {
		return nil
	}
	width, ok := t.newWidth(n, changes)
	if !ok || !t.fits(t.headOf(sep, nil), width, n) {
		return nil
	}
	return t.gap(n)
}

// keyJoin is the break before a brace item's value when c narrows its key or name and the value,
// alone on the line after the `:`, now fits beside it (a Rename of the key, or of what it names).
func (t *canonTree) keyJoin(c format.Change, changes []format.Change) []region {
	value, ok := t.valueOfKey(c.Node)
	if !ok {
		return nil
	}
	sep, broken := t.breakBefore(value)
	if _, list := value.(*syntax.ListLit); list && !broken { // `key: [`, its brackets alone broken
		return t.joinList(value, changes, t.headOf(t.f.Tokens[value.First()-1], &c), false)
	}
	if !broken {
		return nil
	}
	width, ok := t.newWidth(value, changes)
	if !ok || !t.fits(t.headOf(sep, &c), width, value) {
		return nil
	}
	return t.gap(value)
}

// valueOfKey is the value of the map or named item whose key or name holds n.
func (t *canonTree) valueOfKey(n syntax.Node) (syntax.Node, bool) {
	for up := t.parent[n]; up != nil; n, up = up, t.parent[up] {
		switch item := up.(type) {
		case *syntax.MapItem:
			return item.Value, syntax.Node(item.Key) == n
		case *syntax.FieldItem:
			return item.Value, syntax.Node(item.Name) == n
		}
	}
	return nil, false
}

// listJoin is list l printed again, with the break before it, when it is broken over lines only
// by its width, holds no comment, and its new elements fit on its line.
func (t *canonTree) listJoin(l syntax.Node, changes []format.Change) []region {
	src := t.f.Src.Content
	start := int(t.f.Span(l).Start)
	head := string(src[lineStart(src, start):start])
	sep, broken := t.breakBefore(l)
	if broken {
		head = t.headOf(sep, nil)
	}
	return t.joinList(l, changes, head, broken)
}

// joinList is listJoin with the text before l once joined, head, and whether a break precedes l.
func (t *canonTree) joinList(l syntax.Node, changes []format.Change, head string, broken bool) []region {
	if t.oneLine(l) || !t.flattable(l, changes) {
		return nil
	}
	width, ok := t.newWidth(l, changes)
	if !ok || !t.fits(head, width, l) {
		return nil
	}
	out := []region{t.reprint(l)}
	if broken {
		out = append(out, t.gap(l)...)
	}
	return out
}

// flattable reports list l laid on one line once its width allows: no comment inside its brackets,
// each element on one line already, printed again by a change, or such a list itself.
func (t *canonTree) flattable(l syntax.Node, changes []format.Change) bool {
	if t.commentedInside(l) {
		return false
	}
	return !slices.ContainsFunc(t.items(l), func(e syntax.Node) bool { return !t.flatItem(e, changes) })
}

// flatItem reports an element of a list on one line, printed again by a change, or a list that
// itself lays on one line.
func (t *canonTree) flatItem(e syntax.Node, changes []format.Change) bool {
	if t.oneLine(e) || changedAt(e, changes) {
		return true
	}
	_, list := e.(*syntax.ListLit)
	return list && t.flattable(e, changes)
}

// newWidth is the width of n printed flat after changes: its flat text's, each change within it,
// outermost, adding its new text and taking away what it replaces or removes; false where some
// part has no flat text, a new text of several lines included.
func (t *canonTree) newWidth(n syntax.Node, changes []format.Change) (int, bool) {
	flat, err := format.Flat(t.f, n)
	if err != nil {
		return 0, false
	}
	width := utf8.RuneCount(flat)
	for i := range changes {
		d, ok := t.widthChange(i, n, changes)
		if !ok {
			return 0, false
		}
		width += d
	}
	return width, true
}

// widthChange is how much change c widens n, which holds it or not; false where it cannot be
// counted on one line.
func (t *canonTree) widthChange(i int, n syntax.Node, changes []format.Change) (int, bool) {
	c := changes[i]
	switch c.Kind {
	case format.Insert:
		l := t.listOf(c)
		if l == nil || l == syntax.Node(t.f) || !t.within(l, n) {
			return 0, true
		}
		return utf8.RuneCountInString(c.Text) + t.separator(l, i, changes), !strings.Contains(c.Text, newline)
	case format.Replace, format.Remove:
		if !t.within(c.Node, n) || t.insideChange(c, changes) {
			return 0, true
		}
	default:
		return 0, true
	}
	old, err := format.Flat(t.f, c.Node)
	switch {
	case err != nil:
		return 0, false
	case c.Kind == format.Remove:
		return -utf8.RuneCount(old) - t.separator(t.parent[c.Node], i, changes), true
	}
	return utf8.RuneCountInString(c.Text) - utf8.RuneCount(old), !strings.Contains(c.Text, newline)
}

// separator is the width the Insert or Remove changes[i] adds to or takes from list l around its
// item: a separator, or the spaces inside a brace list it leaves alone or empty; none for the
// first item into an empty `[ ]` list, or the first of the removals that empty one.
func (t *canonTree) separator(l syntax.Node, i int, changes []format.Change) int {
	if _, bracket := l.(*syntax.ListLit); !bracket {
		return len(listSep)
	}
	c, first, same := changes[i], -1, 0
	for j, o := range changes {
		if o.Kind != c.Kind || t.listOf(o) != l {
			continue
		}
		if first < 0 {
			first = j
		}
		same++
	}
	items := len(t.items(l))
	empty := c.Kind == format.Insert && items == 0 || c.Kind == format.Remove && items == same
	if empty && i == first {
		return 0
	}
	return len(listSep)
}

// narrowing reports a change that can narrow what holds it: a one-line Replace, a Remove.
func narrowing(c format.Change) bool {
	return c.Kind == format.Remove || c.Kind == format.Replace && !strings.Contains(c.Text, newline)
}

// fits reports n, width wide once flat, fitting the formatter's width after head and before what
// follows it on its line, a trailing comment included.
func (t *canonTree) fits(head string, width int, n syntax.Node) bool {
	src := t.f.Src.Content
	end := t.f.Span(n).End
	tail := strings.TrimSuffix(string(src[end:endOfLine(src, int(end))]), newline)
	return utf8.RuneCountInString(head)+width+utf8.RuneCountInString(tail) <= format.Width
}

// headOf is the line before a node joined after sep: sep's line up to it, then a space; with c,
// the node c replaces on that line written as c's new text.
func (t *canonTree) headOf(sep syntax.Token, c *format.Change) string {
	src := t.f.Src.Content
	lo := lineStart(src, int(sep.End))
	head := string(src[lo:sep.End]) + space
	if c == nil {
		return head
	}
	sp := t.f.Span(c.Node)
	if int(sp.Start) < lo {
		return head
	}
	return string(src[lo:sp.Start]) + c.Text + string(src[sp.End:sep.End]) + space
}

// breakBefore is the `:` or `=` before n, true when n starts a line of its own after it.
func (t *canonTree) breakBefore(n syntax.Node) (syntax.Token, bool) {
	first := n.First()
	if first <= 1 {
		return syntax.Token{}, false
	}
	sep := t.f.Tokens[first-1]
	between := t.f.Src.Content[sep.End:t.f.Tokens[first].Start]
	ok := (sep.Kind == syntax.TokColon || sep.Kind == syntax.TokAssign) && strings.Contains(string(between), newline)
	return sep, ok
}

// gap is the break before n, from its `:` or `=` to its first token.
func (t *canonTree) gap(n syntax.Node) []region {
	sep, _ := t.breakBefore(n)
	return []region{{lo: int(sep.End), hi: int(t.f.Tokens[n.First()].Start), kind: regionAny}}
}

// commentedInside reports a comment between n's first and last tokens: one before n or after
// it on its line is no part of its brackets.
func (t *canonTree) commentedInside(n syntax.Node) bool {
	var tr []syntax.Trivia
	for i := n.First(); i <= n.Last(); i++ {
		tr = tr[:0]
		if i > n.First() {
			tr = t.f.AppendLeading(tr, i)
		}
		if i < n.Last() {
			tr = t.f.AppendTrailing(tr, i)
		}
		if slices.ContainsFunc(tr, comment) {
			return true
		}
	}
	return false
}

// within reports m inside n, or n itself.
func (t *canonTree) within(m, n syntax.Node) bool {
	return n.First() <= m.First() && m.Last() <= n.Last()
}

// insideChange reports c's node inside another change's node, which counts it.
func (t *canonTree) insideChange(c format.Change, changes []format.Change) bool {
	for _, o := range changes {
		if o.Kind == format.Replace && o.Node != c.Node && t.within(c.Node, o.Node) {
			return true
		}
	}
	return false
}

// changedAt reports e the node a Replace prints again.
func changedAt(e syntax.Node, changes []format.Change) bool {
	for _, c := range changes {
		if c.Kind == format.Replace && c.Node == e {
			return true
		}
	}
	return false
}
