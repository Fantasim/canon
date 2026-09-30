package format

import (
	"github.com/fantasim/canonlang/internal/syntax"
)

// focus is a build around lo to hi. A broken brace list's single-line bit forces its group,
// forced and hard rise through cat, indent, group, rhs and ifBreak, so its item sections lie
// between BREAK-mode lines no fits look-ahead crosses: each prints alone, as the file prints it.
type focus struct {
	lo, hi   int // FORMATTER.md §6.1, §7.1
	sections map[syntax.Node]section
}

// section is an item's part of its list or file, after the line break before it; joined when
// a comment that follows it may go on its last line.
type section struct {
	d      *doc
	joined bool
}

// focusOn builds only the items holding a byte of lo to hi: an item wholly outside keeps its
// place in its list but gets no document. Every section is noted.
func (b *builder) focusOn(s span) {
	b.focus = &focus{lo: s.lo, hi: s.hi, sections: map[syntax.Node]section{}}
}

// skips reports an item wholly outside the focus.
func (b *builder) skips(n syntax.Node) bool {
	if b.focus == nil {
		return false
	}
	lo, hi := b.span(n)
	return hi < b.focus.lo || lo > b.focus.hi
}

// notePart notes item n's section d under a focus.
func (b *builder) notePart(n syntax.Node, d *doc, joined bool) {
	if b.focus != nil {
		b.focus.sections[n] = section{d: d, joined: joined}
	}
}

// joinedAfter reports a document starting with a comment written on the line before it.
func joinedAfter(d *doc) bool {
	for d != nil && d.kind == kConcat && len(d.kids) > 0 {
		d = d.kids[0]
	}
	return d != nil && d.kind == kComment && d.alt
}

// fork is a builder of the same file sharing b's reading of its comments and commas.
func (b *builder) fork() *builder {
	c := *b
	c.glued, c.idx, c.focus = map[*syntax.FieldDecl]bool{}, newIndex(), nil
	c.fresh = func(syntax.Tok) bool { return false }
	return &c
}

// brokenKids are the documents under d the printer lays out broken without measuring them, and
// how much deeper: a group forced or holding a line break, rule A whose value is one or keeps
// its brackets, the broken side of an ifBreak; nothing under flat.
var brokenKids = [kindCount]func(d *doc) ([]*doc, int){
	kConcat: func(d *doc) ([]*doc, int) { return d.kids, 0 },
	kIndent: func(d *doc) ([]*doc, int) { return d.kids, indentUnit },
	kGroup: func(d *doc) ([]*doc, int) {
		if d.hard || d.forced {
			return d.kids, 0
		}
		return nil, 0
	},
	kRHS: func(d *doc) ([]*doc, int) {
		if value := d.kids[1]; d.alt || value.hard || value.forced {
			return d.kids, 0
		}
		return nil, 0
	},
	kIfBreak: func(d *doc) ([]*doc, int) { return d.kids[:1], 0 },
}

// broken are the sections reached from root through documents laid out broken without being
// measured, each with the indentation it starts at.
func (x *focus) broken(root *doc) map[*doc]int {
	// FORMATTER.md §7.1
	want := make(map[*doc]bool, len(x.sections))
	for _, s := range x.sections { //canon:unordered builds a set
		want[s.d] = true
	}
	out := map[*doc]int{}
	stack := []cmd{{d: root}}
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if want[c.d] {
			out[c.d] = c.ind
		}
		if next := brokenKids[c.d.kind]; next != nil {
			kids, step := next(c.d)
			for _, k := range kids {
				stack = append(stack, cmd{ind: c.ind + step, d: k})
			}
		}
	}
	return out
}

// renderSection prints a section alone from the start of its line at ind, through the line
// break that ends it.
func renderSection(d *doc, ind int) []byte {
	// FORMATTER.md §13 step 1
	p := &printer{atStart: true, col: ind, lineInd: ind, pendInd: ind}
	p.run(cmd{ind, breakMode, cat(d, hardlineDoc)})
	return p.out
}

// laidOut is the smallest item holding lo to hi whose section prints alone; nil for none.
func (b *builder) laidOut(root *doc, lo, hi int) syntax.Node {
	broken := b.focus.broken(root)
	for n := b.unitAt(lo, hi); n != nil; n = b.outer(n) {
		s, noted := b.focus.sections[n]
		if _, alone := broken[s.d]; noted && alone && !s.joined {
			return n
		}
	}
	return nil
}

// placed is an item's section in a file: its document, the indentation it starts at, its bytes.
type placed struct {
	d     *doc
	ind   int
	bytes span
}

// place is item n's section in b's file, whose document is root; false when it does not print
// alone or its bytes are not whole lines.
func (b *builder) place(root *doc, n syntax.Node) (placed, bool) {
	s, noted := b.focus.sections[n]
	ind, alone := b.focus.broken(root)[s.d]
	at, lines := b.sectionBytes(n)
	return placed{d: s.d, ind: ind, bytes: at}, noted && alone && !s.joined && lines
}

// sectionBytes is where the section of item n lies in the text: from the line after the token
// before n to the end of the line of n's last token, or of the comma after it.
func (b *builder) sectionBytes(n syntax.Node) (span, bool) {
	// FORMATTER.md §4, DECISIONS 216
	prev, last := b.before(n.First()), n.Last()
	if c := b.after(last); b.f.Tokens[c].Kind == syntax.TokComma {
		last = c
	}
	src := b.f.Src.Content
	s := span{lineEnd(src, int(b.f.Tokens[prev].End)) + 1, lineEnd(src, int(b.f.Tokens[last].End)) + 1}
	return s, prev > syntax.NoTok && s.lo <= int(b.f.Tokens[n.First()].Start) && s.hi <= len(src)
}
