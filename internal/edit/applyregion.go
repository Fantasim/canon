package edit

import (
	"bytes"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
)

// region is a byte range of a file before a write, and what the write may put there (API.md M6).
type region struct {
	lo, hi         int
	kind           regionKind
	fromLo, fromHi int // regionMoved: the moved item's own lines in the file before
	comma          int // regionMoved: the offset past its last token, where its comma may change
}

// writeStep is one write of a file: its bytes before and after, the regions of before its
// changes write, outside which every byte is kept (API.md M6), whether it only printed existing
// nodes again (M1, M2), and the operation that made it, cascadeStep for a cascade, once stamped.
type writeStep struct {
	before, after []byte
	regions       []region
	reprint       bool
	op            int
	stamped       bool
}

// reprinted notes whether s's last write only printed existing nodes again (API.md M6 tests).
func (s *fileState) reprinted(only func() bool) {
	if recordWrites && len(s.steps) > 0 {
		s.steps[len(s.steps)-1].reprint = only()
	}
}

// stamp marks each write not yet marked with the operation being applied (API.md M6 tests).
func (a *applier) stamp() {
	if !recordWrites {
		return
	}
	for _, s := range a.files { //canon:unordered marks each file's own writes, in place
		for i := range s.steps {
			if !s.steps[i].stamped {
				s.steps[i].op, s.steps[i].stamped = a.step, true
			}
		}
	}
}

// recordWrites keeps each file's write steps, for the package's tests of API.md M6 only.
var recordWrites bool

// wrote records a write of s, its regions computed only when recorded; no regions is the
// whole file (a normalization, a creation).
func (s *fileState) wrote(after []byte, regions func() []region) {
	if recordWrites {
		rs := []region{{lo: 0, hi: len(s.cur), kind: regionAny}}
		if regions != nil {
			rs = regions()
		}
		s.steps = append(s.steps, writeStep{before: s.cur, after: after, regions: rs})
	}
	s.cur = after
	if after == nil {
		s.origin = ""
	}
}

// canonTree indexes a .canon tree for its regions: each node's parent, in a walk's order, and
// how many items each list gets and loses in the write.
type canonTree struct {
	f       *syntax.File
	parent  map[syntax.Node]syntax.Node
	nodes   []syntax.Node
	lists   map[format.ChangeKind]map[syntax.Node]int // Insert and Remove changes, by list
	leaving map[syntax.Node]bool                      // items removed or moved
}

func newCanonTree(f *syntax.File) *canonTree {
	lists := map[format.ChangeKind]map[syntax.Node]int{format.Insert: {}, format.Remove: {}}
	t := &canonTree{f: f, parent: map[syntax.Node]syntax.Node{}, lists: lists, leaving: map[syntax.Node]bool{}}
	var visit func(n syntax.Node)
	visit = func(n syntax.Node) {
		for c := range syntax.Children(n) {
			if c != nil {
				t.parent[c] = n
				t.nodes = append(t.nodes, c)
				visit(c)
			}
		}
	}
	visit(f)
	return t
}

// canonRegions are the regions format.Rewrite of changes on f writes (FORMATTER.md §13).
func canonRegions(f *syntax.File, changes []format.Change) []region {
	t := newCanonTree(f)
	for _, c := range changes {
		if byList, ok := t.lists[c.Kind]; ok {
			byList[t.listOf(c)]++
		}
		if c.Kind == format.Remove || c.Kind == format.Move {
			t.leaving[c.Node] = true
		}
	}
	out := make([]region, 0, len(changes))
	for _, c := range changes {
		switch c.Kind {
		case format.Insert:
			out = append(out, t.insertRegion(c))
		case format.Remove:
			out = append(out, t.removeRegion(c.Node))
		case format.Move:
			out = append(out, t.moveRegions(c)...)
		default:
			out = append(out, t.reprint(c.Node))
		}
		out = append(out, t.neighbours(c)...)
	}
	return out
}

// listOf is the list a change inserts into or removes from; the file for a declaration.
func (t *canonTree) listOf(c format.Change) syntax.Node {
	switch {
	case c.Kind == format.Remove:
		return t.parent[c.Node]
	case c.List == syntax.NoTok:
		return t.f
	}
	return t.around(c.List)
}

// reprint is the region of the unit a change re-prints: n, or the largest node on one line
// around it; what it holds after is that node's text.
func (t *canonTree) reprint(n syntax.Node) region {
	n = t.climb(n)
	return region{lo: int(t.f.Tokens[n.First()].Start), hi: int(t.f.Tokens[n.Last()].End), kind: regionNode}
}

// climb is n, or the largest node on one line around it.
func (t *canonTree) climb(n syntax.Node) syntax.Node {
	for {
		up := t.parent[n]
		if up == nil || up == syntax.Node(t.f) || !t.oneLine(up) {
			return n
		}
		n = up
	}
}

// oneLine reports n written on one line.
func (t *canonTree) oneLine(n syntax.Node) bool {
	sp := t.f.Span(n)
	return !bytes.Contains(t.f.Src.Content[sp.Start:sp.End], []byte(newline))
}

// relaid reports list printed again whole: on one line, a [ ] list laid out by width that
// loses an item, or emptied (log-2026-09-29 M4 U4b-r3).
func (t *canonTree) relaid(list syntax.Node) bool {
	if list == nil || list == syntax.Node(t.f) {
		return false
	}
	_, bracket := list.(*syntax.ListLit)
	gone := t.lists[format.Remove][list]
	emptied := gone >= len(t.items(list)) && t.lists[format.Insert][list] == 0
	return t.oneLine(list) || gone > 0 && (bracket || emptied)
}

// removeRegion is where removing item n writes: its list printed again, or the item's lines
// with its comments and the one blank line a removal takes with it.
func (t *canonTree) removeRegion(n syntax.Node) region {
	if list := t.parent[n]; t.relaid(list) {
		return t.reprint(list)
	}
	return t.itemLines(t.owned(n))
}

// insertRegion is where an Insert writes: after the file's last item for a declaration, the
// list printed again, or the point between the lines of the items around the position.
func (t *canonTree) insertRegion(c format.Change) region {
	src := t.f.Src.Content
	if c.List == syntax.NoTok {
		return t.appendRegion()
	}
	list := t.around(c.List)
	switch {
	case list == nil:
		return region{lo: 0, hi: len(src), kind: regionAny}
	case t.relaid(list):
		return t.reprint(list)
	}
	items := t.items(list)
	lo := endOfLine(src, int(t.f.Tokens[c.List].End))
	if c.At > 0 && c.At <= len(items) {
		lo = endOfLine(src, t.owned(items[c.At-1]).hi)
	}
	// FORMATTER.md §8.1, §13 step 4
	hi := lineStart(src, t.lead(list.Last()))
	if c.At < len(items) {
		hi = lineStart(src, t.span(items[c.At]).lo)
	}
	return region{lo: lo, hi: max(lo, hi), kind: regionItem}
}

// appendRegion is where a new declaration goes: the line after the file's last declaration, or
// its header, with the comment ending that line; end-of-file comments stay after it.
func (t *canonTree) appendRegion() region {
	// FORMATTER.md §13 step 4
	at := len(t.f.Src.Content)
	var last syntax.Node
	for c := range syntax.Children(t.f) {
		if c != nil && (last == nil || c.Last() > last.Last()) {
			last = c
		}
	}
	if last != nil {
		at = endOfLine(t.f.Src.Content, t.owned(last).hi)
	}
	return region{lo: at, hi: at, kind: regionItem}
}

// lead is where token tok begins with its leading comments.
func (t *canonTree) lead(tok syntax.Tok) int {
	at := t.f.Tokens[tok]
	for _, tr := range at.Leading {
		if comment(tr) {
			return int(tr.Start)
		}
	}
	return int(at.Start)
}

// around is the smallest node holding token open.
func (t *canonTree) around(open syntax.Tok) syntax.Node {
	var best syntax.Node
	for _, n := range t.nodes {
		if n.First() <= open && open <= n.Last() && (best == nil || n.Last()-n.First() < best.Last()-best.First()) {
			best = n
		}
	}
	return best
}

// open is a list's opening bracket.
func (t *canonTree) open(list syntax.Node) syntax.Tok {
	if b, ok := list.(*syntax.AmendBlock); ok {
		return b.Braces.Open
	}
	return list.First()
}

// items are a list's items: its children after its opening bracket.
func (t *canonTree) items(list syntax.Node) []syntax.Node {
	open := t.open(list)
	var out []syntax.Node
	for c := range syntax.Children(list) {
		if c != nil && c.First() > open {
			out = append(out, c)
		}
	}
	return out
}
