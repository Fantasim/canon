package format

import (
	"bytes"
	"slices"
	"strings"
	"unicode/utf8"
)

// mode is how a command prints: broken or flat.
type mode uint8

// cmd is one pending command of the printer: a document, its indentation and its mode.
type cmd struct {
	ind  int
	mode mode
	d    *doc
}

// eol says why the current line must end before any more text: an own-line comment was
// written on it, or a trailing line comment waits for its line break.
type eol uint8

// printer is the algorithm of the formatter: indentation is written with a line's first text,
// so no line ends with spaces, and at most one empty line is ever written in a row.
type printer struct {
	out      []byte
	col      int
	lineInd  int
	pendInd  int
	atStart  bool
	empties  int
	eol      eol
	eolInd   int
	suffixes []string
	stack    []cmd
}

var printTable [kindCount]func(*printer, cmd)

func init() {
	printTable = [kindCount]func(*printer, cmd){
		kText: func(p *printer, c cmd) { p.write(c.ind, c.d.text) }, kConcat: (*printer).concat,
		kLine: (*printer).line, kSoftline: (*printer).softline,
		kHardline: func(p *printer, c cmd) { p.newline(c.ind) }, kIfBreak: (*printer).ifBreak,
		kIndent: func(p *printer, c cmd) { p.push(cmd{c.ind + indentUnit, c.mode, c.d.kids[0]}) },
		kGroup:  (*printer).group, kFlat: func(p *printer, c cmd) { p.push(cmd{c.ind, flatMode, c.d.kids[0]}) },
		kComment: (*printer).ownLine, kSuffix: (*printer).suffix, kBlank: (*printer).blank,
		kMultiline: (*printer).multiline, kRHS: (*printer).rhs,
	}
}

// render prints root, ending with exactly one line break (FORMATTER.md §2).
func render(root *doc) []byte {
	p := &printer{atStart: true, empties: 1}
	p.push(cmd{0, breakMode, root})
	for len(p.stack) > 0 {
		c := p.stack[len(p.stack)-1]
		p.stack = p.stack[:len(p.stack)-1]
		printTable[c.d.kind](p, c)
	}
	p.flushSuffixes()
	p.out = bytes.TrimRight(p.out, space+newlineText)
	return append(p.out, newlineText...)
}

func (p *printer) push(cs ...cmd) {
	for _, c := range slices.Backward(cs) {
		p.stack = append(p.stack, c)
	}
}

func (p *printer) concat(c cmd) {
	for _, d := range slices.Backward(c.d.kids) {
		p.stack = append(p.stack, cmd{c.ind, c.mode, d})
	}
}

func (p *printer) line(c cmd) {
	switch {
	case c.mode == breakMode:
		p.newline(c.ind)
	case p.eol == eolSuffix:
		p.newline(c.ind)
	case p.eol != eolNone:
		p.endLine(c.ind)
	default:
		p.write(c.ind, space)
	}
}

func (p *printer) softline(c cmd) {
	if c.mode == breakMode {
		p.newline(c.ind)
	}
}

func (p *printer) ifBreak(c cmd) {
	d := c.d.kids[1]
	if c.mode == breakMode {
		d = c.d.kids[0]
	}
	p.push(cmd{c.ind, c.mode, d})
}

// group prints its document flat when it fits in what is left of the line.
func (p *printer) group(c cmd) {
	inner := cmd{c.ind, c.mode, c.d.kids[0]}
	switch {
	case c.mode == flatMode:
	case c.d.hard:
		inner.mode = breakMode
	case p.fits(cmd{c.ind, flatMode, inner.d}, Width-p.nextCol(c.ind)):
		inner.mode = flatMode
	}
	p.push(inner)
}

// nextCol is the column the next text starts at, on a new line when the current one must end.
func (p *printer) nextCol(ind int) int {
	switch p.eol {
	case eolComment:
		return p.eolInd
	case eolSuffix:
		return ind + indentUnit
	default:
		return p.col
	}
}

// write writes s on the current line, after its indentation, or on a new line when the
// current one must end.
func (p *printer) write(ind int, s string) {
	if s == "" {
		return
	}
	if p.eol != eolNone {
		p.endLine(ind)
	}
	if !p.atStart && bytes.HasSuffix(p.out, []byte(space)) {
		s = strings.TrimPrefix(s, space)
	}
	if p.atStart {
		s = strings.TrimLeft(s, space)
	}
	if s == "" {
		return
	}
	if p.atStart {
		p.out = append(p.out, strings.Repeat(space, p.pendInd)...)
		p.lineInd, p.atStart = p.pendInd, false
	}
	p.out = append(p.out, s...)
	p.col += utf8.RuneCountInString(s)
	p.empties = 0
}

// endLine breaks a line that must end: after an own-line comment the next text takes the
// comment's indentation, after a trailing comment it continues one level deeper.
func (p *printer) endLine(ind int) {
	if p.eol == eolComment {
		ind = p.eolInd
	} else {
		ind += indentUnit
	}
	p.newline(ind)
}

// newline ends the current line; a line break on an empty line after an empty line is dropped.
func (p *printer) newline(ind int) {
	p.flushSuffixes()
	p.eol = eolNone
	p.pendInd, p.col = ind, ind
	if p.atStart {
		if p.empties > 0 {
			return
		}
		p.empties++
	}
	p.out = bytes.TrimRight(p.out, space)
	p.out = append(p.out, newlineText...)
	p.atStart = true
}

func (p *printer) flushSuffixes() {
	for _, s := range p.suffixes {
		p.out = append(p.out, space+s...)
	}
	p.suffixes = nil
}

func (p *printer) suffix(c cmd) {
	p.suffixes = append(p.suffixes, c.d.text)
	p.eol = eolSuffix
}

// ownLine writes an own-line comment on a line of its own, or after the comment it shared
// its line with; a block comment's inner lines are not re-indented.
func (p *printer) ownLine(c cmd) {
	if c.d.alt && p.eol == eolComment {
		p.out = append(p.out, space+c.d.text...)
		return
	}
	if !p.atStart || p.eol != eolNone {
		p.newline(c.ind)
	}
	if c.d.flag {
		p.blank(c)
	}
	first, rest, multi := strings.Cut(c.d.text, newlineText)
	p.write(c.ind, first)
	if multi {
		p.out = append(p.out, newlineText+rest...)
		p.col = utf8.RuneCountInString(rest[strings.LastIndex(rest, newlineText)+1:])
	}
	if c.d.tight {
		p.write(c.ind, space)
		return
	}
	p.eol, p.eolInd = eolComment, p.lineInd
}

// blank makes sure an empty line precedes what comes next.
func (p *printer) blank(c cmd) {
	if !p.atStart || p.eol != eolNone {
		p.newline(c.ind)
	}
	if p.empties == 0 {
		p.newline(c.ind)
	}
}

// multiline writes a multiline string, its content and closing re-based on the indentation
// of the line holding its opening, plus one level.
func (p *printer) multiline(c cmd) {
	lines := c.d.lines
	p.write(c.ind, lines[0])
	base := strings.Repeat(space, p.lineInd+indentUnit)
	for _, l := range lines[1:] {
		p.out = append(p.out, newlineText...)
		if l != "" {
			p.out = append(p.out, base+l...)
		}
	}
	last := lines[len(lines)-1]
	p.lineInd += indentUnit
	p.col = p.lineInd + utf8.RuneCountInString(last)
}

// rhs is rule A once its group is broken: a bracketed value breaks its own brackets, another
// goes flat on the next line when it fits there, else it breaks its own groups.
func (p *printer) rhs(c cmd) {
	op, value := c.d.kids[0], c.d.kids[1]
	lead := emptyDoc
	if c.d.flag {
		lead = spaceDoc
	}
	next := c.ind + indentUnit
	switch {
	case c.mode == flatMode:
		p.push(cmd{c.ind, flatMode, cat(lead, op, spaceDoc, value)})
	case !c.d.alt && !value.hard && p.fits(cmd{next, flatMode, value}, Width-next):
		p.push(cmd{c.ind, breakMode, cat(lead, op)}, cmd{next, breakMode, hardlineDoc}, cmd{next, flatMode, value})
	default:
		p.push(cmd{c.ind, breakMode, cat(lead, op, spaceDoc)}, cmd{c.ind, breakMode, value})
	}
}
