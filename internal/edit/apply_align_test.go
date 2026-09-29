package edit_test

import (
	"bytes"
	"cmp"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// checkRegions is API.md M6 on one write: after is before with each region's bytes replaced
// and every other byte kept, blank lines included; a removed item's region holds nothing
// after, an item printed again one node's text, an insertion point exactly its new items.
func checkRegions(t *testing.T, name string, w edit.Write) {
	t.Helper()
	if !keptOutside(t, name, w) {
		t.Errorf("%s: the write changes bytes outside its regions %+v (API.md M6):\n%s\n---\n%s", name, w.Regions, w.Before, w.After)
	}
}

// keptOutside reports w's after made of w's before with only its regions' bytes replaced,
// each as its kind allows.
func keptOutside(t *testing.T, name string, w edit.Write) bool {
	t.Helper()
	a := aligner{w: w, rs: merged(w.Regions), shape: shapeOf(t, name, w.After)}
	return a.from(0, 0)
}

// area is a run of merged regions: its bounds, what it may hold after, and how many new items.
type area struct {
	lo, hi, kind, items int
}

// aligner matches a write's after against its before: after is seg0 X0 seg1 X1 ... segN,
// the segs the bytes of before between its areas and each X what area i became.
type aligner struct {
	w     edit.Write
	rs    []area
	shape shape
}

// seg is the bytes of before between area i-1 and area i.
func (a *aligner) seg(i int) []byte {
	lo, hi := 0, len(a.w.Before)
	if i > 0 {
		lo = a.rs[i-1].hi
	}
	if i < len(a.rs) {
		hi = a.rs[i].lo
	}
	return a.w.Before[lo:hi]
}

// from matches seg i at after offset at, then area i and the rest.
func (a *aligner) from(i, at int) bool {
	s := a.seg(i)
	if !bytes.HasPrefix(a.w.After[at:], s) {
		return false
	}
	at += len(s)
	if i == len(a.rs) {
		return at == len(a.w.After)
	}
	for _, end := range a.ends(a.rs[i], at, a.seg(i+1), i+1 == len(a.rs)) {
		if a.from(i+1, end) {
			return true
		}
	}
	return false
}

// ends are the offsets where area r's text, starting at at, may end: nothing for a removal,
// a node's end for an item printed again, exactly r.items items for new items, any for any.
func (a *aligner) ends(r area, at int, next []byte, last bool) []int {
	switch r.kind {
	case edit.RegionGone:
		return []int{at}
	case edit.RegionNode:
		return a.shape.nodes[at]
	}
	after := a.w.After
	var out []int
	if last {
		out = []int{len(after) - len(next)}
	} else {
		for k := at; k < len(after); k++ {
			i := bytes.Index(after[k:], next)
			if i < 0 {
				break
			}
			k += i
			out = append(out, k)
		}
	}
	if r.kind == edit.RegionItem {
		out = slices.DeleteFunc(out, func(e int) bool { return e < at || a.shape.count(after, at, e) != r.items })
	}
	return out
}

// merged are regions sorted, those that overlap or touch joined.
func merged(rs []edit.Region) []area {
	rs = slices.Clone(rs)
	slices.SortFunc(rs, func(x, y edit.Region) int { return x.Lo - y.Lo })
	var out []area
	for _, r := range rs {
		ar := area{lo: r.Lo, hi: r.Hi, kind: r.Kind}
		if r.Kind == edit.RegionItem {
			ar.items = 1
		}
		n := len(out) - 1
		if n < 0 || r.Lo > out[n].hi {
			out = append(out, ar)
			continue
		}
		out[n] = join(out[n], ar)
	}
	return out
}

// join is two touching areas as one: any text if either is, removals and insertions the new
// items alone, a node holding the other that node printed again, else one node printed again.
func join(x, y area) area {
	u := area{lo: min(x.lo, y.lo), hi: max(x.hi, y.hi), kind: edit.RegionNode, items: x.items + y.items}
	edits := func(k int) bool { return k == edit.RegionGone || k == edit.RegionItem }
	switch {
	case x.kind == edit.RegionAny || y.kind == edit.RegionAny:
		u.kind = edit.RegionAny
	case edits(x.kind) && edits(y.kind) && u.items > 0:
		u.kind = edit.RegionItem
	case edits(x.kind) && edits(y.kind):
		u.kind = edit.RegionGone
	}
	return u
}

// shape is what the after text is made of: the ends of its nodes by start, and the extents of
// its items (nodes and JSON members, their comments included), which new text is counted in.
type shape struct {
	nodes   map[int][]int
	extents [][2]int
}

// count is how many items the text lo to hi of after is, -1 when anything but spaces and
// commas lies between them.
func (s shape) count(after []byte, lo, hi int) int {
	var in [][2]int
	for _, e := range s.extents {
		if lo <= e[0] && e[1] <= hi && e[0] < e[1] {
			in = append(in, e)
		}
	}
	slices.SortFunc(in, func(x, y [2]int) int { return cmp.Or(cmp.Compare(x[0], y[0]), cmp.Compare(y[1], x[1])) })
	n, covered := 0, lo
	for _, e := range in {
		if e[0] < covered {
			continue
		}
		if !filler(after[covered:e[0]]) {
			return -1
		}
		n, covered = n+1, e[1]
	}
	if !filler(after[covered:hi]) {
		return -1
	}
	return n
}

func filler(b []byte) bool {
	return strings.Trim(string(b), " \t\n,") == ""
}

// shapeOf parses after as its file's kind.
func shapeOf(t *testing.T, name string, after []byte) shape {
	t.Helper()
	out := shape{nodes: map[int][]int{}}
	var fs source.FileSet
	src, err := fs.Add(name, "/"+name, after)
	if err != nil || len(after) == 0 {
		return out
	}
	bag := diag.NewBag(&fs, "")
	if path.Ext(name) == ".json" {
		if root, err := jsonsrc.Parse(src, bag); err == nil {
			out.json(root)
		}
		return out
	}
	kind := syntax.FileSource
	if path.Base(name) == "project.canon" {
		kind = syntax.FileProject
	}
	out.canon(syntax.Parse(src, kind, bag))
	return out
}

func (s *shape) canon(f *syntax.File) {
	var visit func(n syntax.Node)
	visit = func(n syntax.Node) {
		for c := range syntax.Children(n) {
			if c == nil {
				continue
			}
			first, last := f.Tokens[c.First()], f.Tokens[c.Last()]
			lo, hi := int(first.Start), int(last.End)
			s.nodes[lo] = append(s.nodes[lo], hi)
			for _, tr := range first.Leading {
				if tr.Kind != syntax.TriviaSpace && tr.Kind != syntax.TriviaNewline {
					lo = min(lo, int(tr.Start))
				}
			}
			for _, tr := range last.Trailing {
				if tr.Kind != syntax.TriviaSpace && tr.Kind != syntax.TriviaNewline {
					hi = max(hi, int(tr.End))
				}
			}
			s.extents = append(s.extents, [2]int{lo, hi})
			visit(c)
		}
	}
	visit(f)
}

func (s *shape) json(n *jsonsrc.Node) {
	lo, hi := int(n.Span.Start), int(n.Span.End)
	s.nodes[lo] = append(s.nodes[lo], hi)
	s.extents = append(s.extents, [2]int{lo, hi})
	for _, m := range n.Members {
		klo, khi := int(m.KeySpan.Start), int(m.KeySpan.End)
		s.nodes[klo] = append(s.nodes[klo], khi)
		s.extents = append(s.extents, [2]int{klo, int(m.Value.Span.End)})
		s.json(m.Value)
	}
	for _, e := range n.Elems {
		s.json(e)
	}
}
