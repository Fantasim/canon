package jsonsrc

import (
	"strings"
	"unicode/utf16"

	"github.com/fantasim/canonlang/internal/diag"
)

// hexValue is the value of each hexadecimal digit byte, -1 for any other byte.
var hexValue [byteValues]rune

func init() {
	for c := range hexValue {
		hexValue[c] = -1
	}
	for v := range rune(len(hexDigits)) {
		hexValue[hexDigits[v]] = v
		hexValue[hexDigitsUpper[v]] = v
	}
}

// scalar is the node of the token from start to the current position, as written.
func (p *parser) scalar(k Kind, start int, at link) *Node {
	return &Node{Kind: k, Span: p.span(start, p.pos), Text: string(p.src[start:p.pos]), at: at}
}

func (p *parser) literal(at link) *Node {
	start := p.pos
	for _, l := range literalWords {
		if l.word[0] != p.src[start] {
			continue
		}
		for i := range len(l.word) {
			if !p.accept(l.word[i]) {
				p.fail(p.pos)
				return nil
			}
		}
		return &Node{Kind: l.kind, Span: p.span(start, p.pos), Text: l.word, at: at}
	}
	return p.unexpected(at)
}

// number reads an RFC 8259 §6 number and keeps its text, never a float (WIRE.md §3.3).
func (p *parser) number(at link) *Node {
	start := p.pos
	p.accept(minus)
	if !p.accept(zero) && !p.digits() {
		return nil
	}
	if p.accept(dot) && !p.digits() {
		return nil
	}
	if p.at(exponents[0]) || p.at(exponents[1]) {
		p.pos++
		if p.at(signs[0]) || p.at(signs[1]) {
			p.pos++
		}
		if !p.digits() {
			return nil
		}
	}
	return p.scalar(Number, start, at)
}

// digits consumes one or more decimal digits, or fails at the current position.
func (p *parser) digits() bool {
	start := p.pos
	for p.pos < len(p.src) && p.src[p.pos] >= zero && p.src[p.pos] <= nine {
		p.pos++
	}
	if p.pos == start {
		p.fail(p.pos)
		return false
	}
	return true
}

func (p *parser) stringValue(at link) *Node {
	start := p.pos
	s, ok := p.str()
	if !ok {
		return nil
	}
	return &Node{Kind: String, Span: p.span(start, p.pos), Text: s, at: at}
}

// str reads the string token at the current position and decodes it (WIRE.md §3.2).
func (p *parser) str() (string, bool) {
	start := p.pos
	p.pos++
	escaped := false
	for p.pos < len(p.src) {
		switch c := p.src[p.pos]; {
		case c == quote:
			p.pos++
			return p.decode(start, escaped)
		case c == escapeLead:
			if !p.escape() {
				return "", false
			}
			escaped = true
		case c < controlLimit:
			p.fail(p.pos)
			return "", false
		default:
			p.pos++
		}
	}
	p.fail(p.pos)
	return "", false
}

// decode is the value of the string token from start to the current position; str has
// validated it, so diag's codec (DECISIONS 85) decodes its escapes without an error.
func (p *parser) decode(start int, escaped bool) (string, bool) {
	if !escaped {
		return string(p.src[start+1 : p.pos-1]), true
	}
	s, _, _ := diag.UnquoteJSON(string(p.src[start:p.pos]))
	return s, true
}

// escape validates the escape at the current position, surrogates paired (WIRE.md §3.2).
func (p *parser) escape() bool {
	start := p.pos
	p.pos++
	if p.at(unicodeLetter) {
		p.pos++
		return p.unicodeEscape(start)
	}
	if p.pos < len(p.src) && strings.IndexByte(shortEscapes, p.src[p.pos]) >= 0 {
		p.pos++
		return true
	}
	p.fail(p.pos)
	return false
}

func (p *parser) unicodeEscape(start int) bool {
	r, ok := p.hex4()
	if !ok || !utf16.IsSurrogate(r) {
		return ok
	}
	if r < surrogateLow && p.lowSurrogateAt(p.pos) {
		p.pos += hexEscapeWidth
		return true
	}
	p.err = &EncodingError{Span: p.span(start, start+hexEscapeWidth), Surrogate: r}
	return false
}

// hex4 reads the four hexadecimal digits of a \u escape, or fails at the first other byte.
func (p *parser) hex4() (rune, bool) {
	digits := p.src[p.pos:min(p.pos+hexEscapeDigits, len(p.src))]
	r, bad := hexRune(digits)
	if bad < 0 && len(digits) < hexEscapeDigits {
		bad = len(digits)
	}
	if bad >= 0 {
		p.fail(p.pos + bad)
		return 0, false
	}
	p.pos += hexEscapeDigits
	return r, true
}

// lowSurrogateAt reports whether the source holds the \u escape of a low surrogate at pos.
func (p *parser) lowSurrogateAt(pos int) bool {
	if pos+hexEscapeWidth > len(p.src) || p.src[pos] != escapeLead || p.src[pos+1] != unicodeLetter {
		return false
	}
	r, bad := hexRune(p.src[pos+escapePrefixWidth : pos+hexEscapeWidth])
	return bad < 0 && r >= surrogateLow && utf16.IsSurrogate(r)
}

// hexRune is the value of hexadecimal digits, and -1, or the index of the first other byte.
func hexRune(digits []byte) (rune, int) {
	var r rune
	for i, c := range digits {
		if hexValue[c] < 0 {
			return 0, i
		}
		r = r<<hexBits | hexValue[c]
	}
	return r, -1
}
