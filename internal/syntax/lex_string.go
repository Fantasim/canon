package syntax

import (
	"github.com/fantasim/canonlang/internal/diag"
)

// stringKinds are the token kinds of a string piece, by [multiline][opened by a quote][ends at
// a quote]: a whole string, a head, a middle or a tail (DECISIONS 73).
var stringKinds = [boolCount][boolCount][boolCount]TokenKind{
	{{TokStringMid, TokStringTail}, {TokStringHead, TokString}},
	{{TokMLStringMid, TokMLStringTail}, {TokMLStringHead, TokMLString}},
}

// strScan is one piece of a string being read: its start (the quote, else an interpolation's
// "}"), whether a quote opened it, the multiline state (nil for a plain string) and the string's
// opening quote, where an E1107 goes whatever piece breaks off.
type strScan struct {
	start int
	first bool
	ml    *mlString
	quote int
}

// openString is the first piece of the string whose opening quote is at start.
func openString(start int, ml *mlString) strScan {
	return strScan{start: start, first: true, ml: ml, quote: start}
}

// stringText reads string text from pos, the piece s, up to the closing quote or the next "{".
func (l *lexer) stringText(s strScan) {
	for l.pos < len(l.src) {
		if done := stringByte[stringClass(l.src[l.pos])](l, &s); done {
			return
		}
	}
	diag.E1107.At(l.span(s.quote, s.quote+1)).Report(l.bag)
	l.endPiece(&s, true)
}

// stringByte handles a byte of string text by class; it reports whether the piece ended.
var stringByte [strClassCount]func(*lexer, *strScan) bool

func init() {
	stringByte = [strClassCount]func(*lexer, *strScan) bool{
		strPlain: (*lexer).strPlain, strQuote: (*lexer).strQuote, strNewline: (*lexer).strNewline,
		strEscape: (*lexer).strEscape, strOpen: (*lexer).strOpen, strClose: (*lexer).strClose,
	}
}

func stringClass(c byte) int {
	switch c {
	case '"':
		return strQuote
	case '\n':
		return strNewline
	case '\\':
		return strEscape
	case '{':
		return strOpen
	case '}':
		return strClose
	}
	return strPlain
}

func (l *lexer) strPlain(*strScan) bool {
	end := l.runeEnd(l.pos)
	l.checkText(l.pos, end, false)
	l.pos = end
	return false
}

func (l *lexer) strQuote(s *strScan) bool {
	if s.ml == nil {
		l.pos++
		l.endPiece(s, true)
		return true
	}
	if !hasPrefixAt(l.src, l.pos, tripleQuote) {
		l.pos++
		return false
	}
	l.closeML(s.ml)
	l.endPiece(s, true)
	return true
}

func (l *lexer) strNewline(s *strScan) bool {
	if s.ml == nil {
		diag.E1107.At(l.span(s.quote, l.pos)).Report(l.bag)
		l.endPiece(s, true)
		return true
	}
	l.pos++
	s.ml.lines = append(s.ml.lines, l.pos)
	return false
}

// strEscape checks one escape (GRAMMAR.md §2.6): \n \t \r \\ \" \{ \} or \u{H…}.
func (l *lexer) strEscape(*strScan) bool {
	start := l.pos
	next := l.peek(1)
	switch {
	case isSimpleEscape(next):
		l.pos += pairWidth
	case next == 'u' && l.peek(pairWidth) == '{':
		l.pos = l.unicodeEscape(start)
	default:
		l.pos++
		if next != '\n' && l.pos < len(l.src) {
			l.pos = l.runeEnd(l.pos)
		}
		diag.E1109.At(l.span(start, l.pos), l.text(start, l.pos)).Report(l.bag)
	}
	return false
}

// unicodeEscape checks "\u{H…}": 1 to 6 hex digits naming a scalar value; it returns its end.
func (l *lexer) unicodeEscape(start int) int {
	digits := start + unicodeOpen
	i := digits
	for i < len(l.src) && isHexDigit(l.src[i]) {
		i++
	}
	hex := l.src[digits:i]
	closed := i < len(l.src) && l.src[i] == '}'
	if closed {
		i++
	}
	if !closed || !validScalar(hex) {
		diag.E1109.At(l.span(start, i), l.text(start, i)).Report(l.bag)
	}
	return i
}

func (l *lexer) strOpen(s *strScan) bool {
	if l.peek(1) == '{' {
		l.pos += pairWidth
		return false
	}
	l.pos++
	l.endPiece(s, false)
	l.frames = append(l.frames, interpFrame{ml: s.ml, quote: s.quote, first: len(l.toks), open: l.pos - 1})
	return true
}

func (l *lexer) strClose(*strScan) bool {
	if l.peek(1) == '}' {
		l.pos += pairWidth
		return false
	}
	diag.E1112.AtBrace(l.span(l.pos, l.pos+1)).Report(l.bag)
	l.pos++
	return false
}

// endPiece emits the piece that ends at pos: at a quote (or where it broke off), or at "{".
func (l *lexer) endPiece(s *strScan, quote bool) {
	ml := 0
	if s.ml != nil {
		ml = 1
	}
	l.emit(stringKinds[ml][boolIndex(s.first)][boolIndex(quote)], s.start)
}

// rbrace is "}": the end of an interpolation at depth 0, back to the string's text.
func (l *lexer) rbrace() {
	if !l.inInterp() || l.top().depth > 0 {
		l.punct()
		return
	}
	f := l.frames[len(l.frames)-1]
	l.frames = l.frames[:len(l.frames)-1]
	if f.first == len(l.toks) {
		diag.E1112.AtEmpty(l.span(f.open, l.pos+1)).Report(l.bag)
	}
	l.pos++
	l.stringText(strScan{start: l.pos - 1, ml: f.ml, quote: f.quote})
}

// colon is ":": at depth 0 of an interpolation it starts a format spec (GRAMMAR.md §2.6).
func (l *lexer) colon() {
	if !l.inInterp() || l.top().depth > 0 {
		l.punct()
		return
	}
	f := l.top()
	if f.first == len(l.toks) {
		diag.E1112.AtEmpty(l.span(f.open, l.pos+1)).Report(l.bag)
	}
	start := l.pos
	l.pos++
	for l.pos < len(l.src) && !isSpecEnd(l.src[l.pos]) {
		l.pos = l.runeEnd(l.pos)
	}
	l.emit(TokFormatSpec, start)
	spec := l.src[start+1 : l.pos]
	if _, ok := parseSpec(spec); !ok || l.peek(0) != '}' {
		diag.E1101.At(l.span(start, l.pos), string(spec)).Report(l.bag)
	}
}

// interpNewline is a line break inside an interpolation (E1112): a multiline string goes on
// with its text, a plain string is dropped.
func (l *lexer) interpNewline() {
	f := l.frames[len(l.frames)-1]
	l.frames = l.frames[:len(l.frames)-1]
	diag.E1112.AtNewline(l.span(l.pos, l.pos+1)).Report(l.bag)
	if f.ml != nil {
		l.stringText(strScan{start: l.pos, ml: f.ml, quote: f.quote})
	}
}

func isSimpleEscape(c byte) bool {
	switch c {
	case 'n', 't', 'r', '\\', '"', '{', '}':
		return true
	}
	return false
}

func isHexDigit(c byte) bool {
	return isDigit(c) || (c|asciiLowerBit >= 'a' && c|asciiLowerBit <= 'f')
}

func isSpecEnd(c byte) bool { return c == '}' || c == '"' || c == '\n' }

func boolIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}
