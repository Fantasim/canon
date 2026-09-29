package format

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
)

// batch is the Insert, Remove and Move changes of one list, or of the file's top level: which
// items leave their place, which a change names already, and per position 0 to the item count
// what arrives there, in the order of the changes.
type batch struct {
	l                list
	top              bool
	removed, claimed []bool
	news             [][]arrival
}

// arrival is what an Insert or a Move puts at a position: a new text, or item from moved there.
type arrival struct {
	text string
	from int
}

// batches are the batches of an edit, by opening bracket, in the order they first appear.
type batches struct {
	byList map[syntax.Tok]*batch
	order  []syntax.Tok
}

// slot is one item of a list once its group applies: item at, kept, or a new text or item
// from placed at position at; first and last are the kinds of its edge tokens (DECISIONS 211).
type slot struct {
	at, item, from int
	text           string
	first, last    syntax.TokenKind
}

// batchOf is the group of the list open opens, syntax.NoTok for the top level.
func (b *builder) batchOf(gs *batches, open syntax.Tok) (*batch, error) {
	if g, ok := gs.byList[open]; ok {
		return g, nil
	}
	g := &batch{top: open == syntax.NoTok}
	if i, ok := b.idx.byTok[open]; ok && !b.idx.lists[i].partial {
		g.l = b.idx.lists[i]
	} else if ok || !g.top || b.f.FileKind == syntax.FileProject {
		return nil, fmt.Errorf("%w: no list to change", ErrChange)
	} else {
		g.l = list{items: b.tops()}
	}
	g.removed, g.claimed = make([]bool, len(g.l.items)), make([]bool, len(g.l.items))
	g.news = make([][]arrival, len(g.l.items)+1)
	gs.byList[open] = g
	gs.order = append(gs.order, open)
	return g, nil
}

// collect adds an Insert, a Remove or a Move to its group; a declaration is only appended, an
// item moves only within its list.
func (b *builder) collect(gs *batches, c Change) error {
	if c.Kind == Insert {
		return b.collectInsert(gs, c)
	}
	i, known := b.idx.owner[c.Node]
	if !known {
		return fmt.Errorf("%w: not an item", ErrChange)
	}
	open := syntax.NoTok
	if i >= 0 {
		open = b.idx.lists[i].open
	}
	if c.Kind == Move && (i < 0 || c.List != open) {
		return fmt.Errorf("%w: an item moves only within its list", ErrChange)
	}
	g, err := b.batchOf(gs, open)
	if err != nil {
		return err
	}
	at := slices.Index(g.l.items, c.Node)
	if g.claimed[at] {
		return fmt.Errorf("%w: removed or moved twice", ErrChange)
	}
	g.claimed[at] = true
	if c.Kind == Remove {
		g.removed[at] = true
		return nil
	}
	return g.move(at, c.At)
}

// collectInsert adds an Insert to its group.
func (b *builder) collectInsert(gs *batches, c Change) error {
	g, err := b.batchOf(gs, c.List)
	if err != nil {
		return err
	}
	t, n := trimmed(c.Text), len(g.l.items)
	if t == "" || c.At < 0 || c.At > n || g.top && c.At != n {
		return fmt.Errorf("%w: no position %d to insert at", ErrChange, c.At)
	}
	g.news[c.At] = append(g.news[c.At], arrival{text: t, from: -1})
	return nil
}

// final is the items of g's list once its changes apply.
func (b *builder) final(g *batch) []slot {
	var out []slot
	for p, news := range g.news {
		for _, e := range news {
			out = append(out, b.arriving(g, p, e))
		}
		if p < len(g.removed) && !g.removed[p] {
			n := g.l.items[p]
			out = append(out, slot{at: p, item: p, from: -1, first: b.f.Tokens[n.First()].Kind, last: b.f.Tokens[n.Last()].Kind})
		}
	}
	return out
}

// arriving is the slot of e at position p: a new text, read by the lexer, or a moved item.
func (b *builder) arriving(g *batch, p int, e arrival) slot {
	s := slot{at: p, item: -1, from: e.from, text: e.text}
	if e.from < 0 {
		s.first, s.last = edges(e.text)
		return s
	}
	n := g.l.items[e.from]
	s.first, s.last = b.f.Tokens[n.First()].Kind, b.f.Tokens[n.Last()].Kind
	return s
}

// needsComma reports a comma after slot k of a list laid out one item per line: after every
// element of a ( ) or [ ] list, after an item of a brace list that its line would join to the
// next one (DECISIONS 211).
func needsComma(brace bool, fin []slot, k int) bool {
	if !brace {
		return true
	}
	return k+1 < len(fin) && (cannotEndItem[fin[k].last] || joinsLine[fin[k+1].first])
}

// want is what a new text of g must parse as: an item of its list, like its first item.
func (g *batch) want() expect {
	w := expect{check: true, item: true, top: g.top}
	if len(g.l.items) > 0 && !g.top {
		w.like = g.l.items[0]
	}
	return w
}

// plan is the edits of one group (FORMATTER.md §13 steps 4 and 5).
func (b *builder) plan(g *batch) ([]edit, error) {
	if g.top || b.broken(g.l) {
		return b.planLines(g)
	}
	return b.planInline(g)
}

// tops are the file's top-level items in source order.
func (b *builder) tops() []syntax.Node {
	var out []syntax.Node
	for _, n := range b.idx.items {
		if b.idx.owner[n] < 0 {
			out = append(out, n)
		}
	}
	slices.SortFunc(out, func(x, y syntax.Node) int { return cmp.Compare(x.First(), y.First()) })
	return out
}

// header is the last token of the file's header: its last import, layer or language, or its
// package name.
func (b *builder) header() syntax.Tok {
	last := b.f.Package.Last()
	for _, n := range []syntax.Node{b.f.Layer, b.f.Lang} {
		if !absent(n) {
			last = max(last, n.Last())
		}
	}
	for _, imp := range b.f.Imports {
		last = max(last, imp.Last())
	}
	return last
}

// categories tell the kinds of node a new text may stand for, the first one the old node
// matches deciding.
var categories = [...]func(syntax.Node) bool{
	func(n syntax.Node) bool { _, ok := n.(syntax.Expr); return ok },
	func(n syntax.Node) bool { _, ok := n.(syntax.Type); return ok },
	func(n syntax.Node) bool { _, ok := n.(syntax.Stmt); return ok },
	func(n syntax.Node) bool { _, ok := n.(syntax.BraceItem); return ok },
	func(n syntax.Node) bool { _, ok := n.(syntax.RecordItem); return ok },
	func(n syntax.Node) bool { _, ok := n.(syntax.VariantItem); return ok },
	func(n syntax.Node) bool { _, ok := n.(syntax.ViewItem); return ok },
	func(n syntax.Node) bool { _, ok := n.(syntax.Decl); return ok },
}
