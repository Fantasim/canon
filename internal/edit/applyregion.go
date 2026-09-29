package edit

import (
	"bytes"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
)

// region is a byte range of a file before a write, and what the write may put there (API.md M6).
type region struct {
	lo, hi int
	kind   regionKind
}

// writeStep is one write of a file: its bytes before and after, and the regions of before its
// changes write; outside them every byte is kept (API.md M6).
type writeStep struct {
	before, after []byte
	regions       []region
}

// recordWrites keeps each file's write steps, for the package's tests of API.md M6 only.
var recordWrites bool

// wrote records a write of s, its regions computed only when recorded; no regions is the
// whole file (a normalization, a creation).
func (s *fileState) wrote(after []byte, regions func() []region) {
	if recordWrites {
		rs := []region{{0, len(s.cur), regionAny}}
		if regions != nil {
			rs = regions()
		}
		s.steps = append(s.steps, writeStep{before: s.cur, after: after, regions: rs})
	}
	s.cur = after
}

// canonTree indexes a .canon tree for its regions: each node's parent, in a walk's order, and
// how many items each list gets and loses in the write.
type canonTree struct {
	f      *syntax.File
	parent map[syntax.Node]syntax.Node
	nodes  []syntax.Node
	lists  map[format.ChangeKind]map[syntax.Node]int // Insert and Remove changes, by list
}

func newCanonTree(f *syntax.File) *canonTree {
	lists := map[format.ChangeKind]map[syntax.Node]int{format.Insert: {}, format.Remove: {}}
	t := &canonTree{f: f, parent: map[syntax.Node]syntax.Node{}, lists: lists}
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
	}
	out := make([]region, 0, len(changes))
	for _, c := range changes {
		switch c.Kind {
		case format.Insert:
			out = append(out, t.insertRegion(c))
		case format.Remove:
			out = append(out, t.removeRegion(c.Node))
		default:
			out = append(out, t.reprint(c.Node))
		}
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
	return region{int(t.f.Tokens[n.First()].Start), int(t.f.Tokens[n.Last()].End), regionNode}
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
	return t.itemLines(t.span(n))
}

// insertRegion is where an Insert writes: the end of the file for a declaration, the list
// printed again, or the point between the lines of the items around the position.
func (t *canonTree) insertRegion(c format.Change) region {
	src := t.f.Src.Content
	if c.List == syntax.NoTok {
		return region{len(src), len(src), regionItem}
	}
	list := t.around(c.List)
	switch {
	case list == nil:
		return region{0, len(src), regionAny}
	case t.relaid(list):
		return t.reprint(list)
	}
	items := t.items(list)
	lo := endOfLine(src, int(t.f.Tokens[c.List].End))
	if c.At > 0 && c.At <= len(items) {
		lo = endOfLine(src, t.span(items[c.At-1]).hi)
	}
	hi := lineStart(src, int(t.f.Tokens[list.Last()].Start))
	if c.At < len(items) {
		hi = lineStart(src, t.span(items[c.At]).lo)
	}
	return region{lo, max(lo, hi), regionItem}
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

// items are a list's items: its children after its opening bracket.
func (t *canonTree) items(list syntax.Node) []syntax.Node {
	open := list.First()
	if b, ok := list.(*syntax.AmendBlock); ok {
		open = b.Braces.Open
	}
	var out []syntax.Node
	for c := range syntax.Children(list) {
		if c != nil && c.First() > open {
			out = append(out, c)
		}
	}
	return out
}
