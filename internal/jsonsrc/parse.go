package jsonsrc

import (
	"bytes"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// valueTable dispatches a value on its first byte (DECISIONS 26).
var valueTable [byteValues]func(*parser, link) *Node

func init() {
	for c := range valueTable {
		valueTable[c] = (*parser).unexpected
	}
	valueTable[openObject], valueTable[openArray] = (*parser).object, (*parser).array
	valueTable[quote], valueTable[minus] = (*parser).stringValue, (*parser).number
	for c := byte(zero); c <= nine; c++ {
		valueTable[c] = (*parser).number
	}
	for _, l := range literalWords {
		valueTable[l.word[0]] = (*parser).literal
	}
}

// parser reads one source; it stops at the first syntax or encoding error.
type parser struct {
	src   []byte
	file  source.FileID
	pos   int
	depth int
	where *Node // the innermost open array or object
	bag   *diag.Bag
	err   error
	dups  []repeat
}

// repeat is a key its object already has: the E7104 to report once the source parses.
type repeat struct {
	member Member
	first  source.Span
}

// Parse reads a JSON source: its tree, or no tree and ErrSyntax after one E7109,
// ErrDuplicateKey after one E7104 per repeated key, or an *EncodingError, reporting nothing.
func Parse(f *source.File, bag *diag.Bag) (*Node, error) {
	// WIRE.md §3
	if err := checkEncoding(f); err != nil {
		return nil, err
	}
	p := &parser{src: f.Content, file: f.ID, bag: bag}
	if bytes.HasPrefix(p.src, []byte(UTF8BOM)) {
		p.pos = len(UTF8BOM)
	}
	p.space()
	root := p.value(link{})
	if p.err == nil {
		p.space()
		if p.pos < len(p.src) {
			p.fail(p.pos)
		}
	}
	if p.err != nil {
		return nil, p.err
	}
	for _, r := range p.dups {
		m := r.member
		diag.E7104.AtJson(m.KeySpan, m.Key, r.first).Pointer(m.Value.Pointer()).Report(bag)
	}
	if len(p.dups) > 0 {
		return nil, ErrDuplicateKey
	}
	return root, nil
}

// checkEncoding refuses UTF-16 and UTF-32 byte order marks and non-UTF-8 bytes (WIRE.md §3.1).
func checkEncoding(f *source.File) error {
	for _, bom := range ForeignBOMs {
		if bytes.HasPrefix(f.Content, []byte(bom)) {
			return &EncodingError{Span: spanOf(f.ID, 0, len(bom))}
		}
	}
	if i := InvalidUTF8At(f.Content); i >= 0 {
		return &EncodingError{Span: spanOf(f.ID, i, i+1)}
	}
	return nil
}

// InvalidUTF8At is the offset of content's first invalid byte, -1 when it is all valid UTF-8 (WIRE.md §3.1).
func InvalidUTF8At(content []byte) int {
	if utf8.Valid(content) {
		return -1
	}
	for i := 0; i < len(content); {
		r, width := utf8.DecodeRune(content[i:])
		if r == utf8.RuneError && width == 1 {
			return i
		}
		i += width
	}
	return -1
}

func (p *parser) span(start, end int) source.Span {
	return spanOf(p.file, start, end)
}

// spanOf is the span of byte offsets; FileSet.Add refuses content whose offsets exceed a Pos.
func spanOf(file source.FileID, start, end int) source.Span {
	return source.Span{File: file, Start: posOf(start), End: posOf(end)}
}

func posOf(i int) source.Pos {
	if i < 0 || i > math.MaxInt32 {
		return math.MaxInt32
	}
	return source.Pos(i)
}

// charAt is the char variant's argument at pos, and its byte width (WIRE.md §7.3, DECISIONS 208).
func (p *parser) charAt(pos int) (string, int) {
	_, width := utf8.DecodeRune(p.src[pos:])
	return string(diag.AppendJSONString(nil, string(p.src[pos:pos+width]))), width
}

// fail reports one E7109 at pos: eof or char, by whether pos is the source's end (DECISIONS 208).
func (p *parser) fail(pos int) {
	if pos >= len(p.src) {
		diag.E7109.AtEof(p.span(pos, pos)).Pointer(p.where.Pointer()).Report(p.bag)
	} else {
		char, width := p.charAt(pos)
		diag.E7109.AtChar(p.span(pos, pos+width), char).Pointer(p.where.Pointer()).Report(p.bag)
	}
	p.err = ErrSyntax
}

// failDepth reports the E7109 depth variant at the opener past maxDepth (DECISIONS 208).
func (p *parser) failDepth(pos int) {
	_, width := p.charAt(pos)
	diag.E7109.AtDepth(p.span(pos, pos+width), maxDepth).Pointer(p.where.Pointer()).Report(p.bag)
	p.err = ErrSyntax
}

func (p *parser) space() {
	for p.pos < len(p.src) && strings.IndexByte(jsonSpace, p.src[p.pos]) >= 0 {
		p.pos++
	}
}

// at reports whether the source has c at the current position.
func (p *parser) at(c byte) bool {
	return p.pos < len(p.src) && p.src[p.pos] == c
}

// accept consumes c when the source has it at the current position.
func (p *parser) accept(c byte) bool {
	if p.at(c) {
		p.pos++
		return true
	}
	return false
}

// expect consumes c, or fails at the current position.
func (p *parser) expect(c byte) bool {
	if p.accept(c) {
		return true
	}
	p.fail(p.pos)
	return false
}

func (p *parser) value(at link) *Node {
	if p.pos >= len(p.src) {
		p.fail(p.pos)
		return nil
	}
	return valueTable[p.src[p.pos]](p, at)
}

func (p *parser) unexpected(link) *Node {
	p.fail(p.pos)
	return nil
}

// open starts an array or object at the current byte, within the nesting limit.
func (p *parser) open(k Kind, at link) *Node {
	if p.depth == maxDepth {
		p.failDepth(p.pos)
		return nil
	}
	p.depth++
	n := &Node{Kind: k, Span: p.span(p.pos, p.pos), at: at}
	p.where = n
	p.pos++
	p.space()
	return n
}

// close ends n after its closing byte, c, and returns to its container.
func (p *parser) close(n *Node, c byte) *Node {
	if !p.expect(c) {
		return nil
	}
	p.depth--
	p.where = n.at.parent
	n.Span.End = posOf(p.pos)
	return n
}

func (p *parser) array(at link) *Node {
	n := p.open(Array, at)
	if n == nil {
		return nil
	}
	if p.at(closeArray) {
		return p.close(n, closeArray)
	}
	for {
		e := p.value(link{parent: n, index: len(n.Elems)})
		if e == nil {
			return nil
		}
		n.Elems = append(n.Elems, e)
		p.space()
		if !p.accept(comma) {
			return p.close(n, closeArray)
		}
		p.space()
	}
}

func (p *parser) object(at link) *Node {
	n := p.open(Object, at)
	if n == nil {
		return nil
	}
	if p.at(closeObject) {
		return p.close(n, closeObject)
	}
	var seen map[string]int
	for {
		m, ok := p.member(n)
		if !ok {
			return nil
		}
		n.Members = append(n.Members, m)
		seen = p.unique(n.Members, seen)
		p.space()
		if !p.accept(comma) {
			return p.close(n, closeObject)
		}
		p.space()
	}
}

func (p *parser) member(obj *Node) (Member, bool) {
	if !p.at(quote) {
		p.fail(p.pos)
		return Member{}, false
	}
	start := p.pos
	key, ok := p.str()
	if !ok {
		return Member{}, false
	}
	m := Member{Key: key, KeySpan: p.span(start, p.pos)}
	p.space()
	if !p.expect(colon) {
		return Member{}, false
	}
	p.space()
	m.Value = p.value(link{parent: obj, key: key})
	return m, m.Value != nil
}

// unique keeps an E7104 when the last of members repeats a key of the ones before it, to
// report once the whole source parses (DECISIONS 164); past a few members, seen indexes keys.
func (p *parser) unique(members []Member, seen map[string]int) map[string]int {
	last := len(members) - 1
	m, before := &members[last], members[:last]
	first := -1
	switch {
	case seen != nil:
		if i, ok := seen[m.Key]; ok {
			first = i
		}
	case last < indexedMembers:
		first = slices.IndexFunc(before, func(b Member) bool { return b.Key == m.Key })
	default:
		seen = make(map[string]int, len(members))
		for i, b := range slices.Backward(before) {
			seen[b.Key] = i
		}
		return p.unique(members, seen)
	}
	if first >= 0 {
		p.dups = append(p.dups, repeat{member: *m, first: before[first].KeySpan})
	} else if seen != nil {
		seen[m.Key] = last
	}
	return seen
}
