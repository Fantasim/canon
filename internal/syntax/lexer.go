package syntax

import (
	"bytes"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// interpFrame is an open interpolation: its bracket depth, the string it belongs to (ml is nil
// for a plain string), its first token and its opening brace.
type interpFrame struct {
	depth int
	ml    *mlString
	first int
	open  int
}

// lexer turns a file into the tokens of GRAMMAR.md §2, each with its trivia (DECISIONS 73).
type lexer struct {
	src      []byte
	file     source.FileID
	bag      *diag.Bag
	toks     []Token
	pos      int
	lead     []Trivia
	trailing bool
	lineText bool
	frames   []interpFrame
}

// lex returns the tokens of src, BOF first and EOF last, without NL separators.
func lex(src *source.File, bag *diag.Bag) []Token {
	l := &lexer{src: src.Content, file: src.ID, bag: bag}
	l.toks = append(l.toks, Token{Kind: TokBOF})
	if bytes.HasPrefix(l.src, []byte(bomText)) {
		l.pos = len(bomText)
		l.trivia(TriviaBOM, 0)
		diag.E1123.At(l.span(0, l.pos)).Report(bag)
	}
	for l.pos < len(l.src) {
		l.step()
	}
	for _, f := range slices.Backward(l.frames) {
		diag.E1112.AtUnterminated(l.span(f.open, f.open+1)).Report(bag)
	}
	end := pos(len(l.src))
	l.toks = append(l.toks, Token{Kind: TokEOF, Start: end, End: end, Leading: l.lead})
	return l.toks
}

// pos is a byte offset as a Pos; FileSet.Add refuses content whose offsets do not fit one.
func pos(i int) source.Pos {
	if i < 0 || i > math.MaxInt32 {
		return math.MaxInt32
	}
	return source.Pos(i)
}

func (l *lexer) step() {
	c := l.src[l.pos]
	if c >= utf8.RuneSelf {
		l.nonASCII()
		return
	}
	codeTable[c](l)
}

func (l *lexer) span(start, end int) source.Span {
	return source.Span{File: l.file, Start: pos(start), End: pos(end)}
}

func (l *lexer) text(start, end int) string { return string(l.src[start:end]) }

// emit ends the token that started at start, giving it the pending leading trivia.
func (l *lexer) emit(k TokenKind, start int) {
	t := Token{Kind: k, Start: pos(start), End: pos(l.pos), Leading: l.lead}
	l.toks = append(l.toks, t)
	l.lead = nil
	l.trailing = true
	l.lineText = true
}

// trivia records [start, pos): on the last token's line it trails that token, from the first
// line break on it leads the next one.
func (l *lexer) trivia(k TriviaKind, start int) {
	t := Trivia{Kind: k, Start: pos(start), End: pos(l.pos)}
	if l.trailing && k != TriviaNewline {
		last := &l.toks[len(l.toks)-1]
		last.Trailing = append(last.Trailing, t)
		return
	}
	l.trailing = false
	l.lead = append(l.lead, t)
}

func (l *lexer) peek(off int) byte {
	if l.pos+off < len(l.src) {
		return l.src[l.pos+off]
	}
	return 0
}

func (l *lexer) inInterp() bool { return len(l.frames) > 0 }

func (l *lexer) top() *interpFrame { return &l.frames[len(l.frames)-1] }

func (l *lexer) prevKind() TokenKind { return l.toks[len(l.toks)-1].Kind }

// nonASCII handles a byte above 0x7F outside literals and comments (GRAMMAR.md §1).
func (l *lexer) nonASCII() {
	start := l.pos
	r, size := utf8.DecodeRune(l.src[l.pos:])
	l.pos += size
	if r == utf8.RuneError && size == 1 {
		diag.E1124.AtUtf8(l.span(start, l.pos)).Report(l.bag)
	} else {
		diag.E1106.At(l.span(start, l.pos), l.text(start, l.pos)).Report(l.bag)
	}
	l.emit(TokIllegal, start)
}

// checkText reports invalid UTF-8 and control characters in [start, end) of a literal or a
// comment; a tab always passes, a line break when lines is set.
func (l *lexer) checkText(start, end int, lines bool) {
	for i := start; i < end; {
		r, size := utf8.DecodeRune(l.src[i:end])
		switch {
		case r == utf8.RuneError && size == 1:
			diag.E1124.AtUtf8(l.span(i, i+size)).Report(l.bag)
		case r == '\r':
			diag.E1124.AtCr(l.span(i, i+size)).Report(l.bag)
		case r < ' ' && r != '\t' && (r != '\n' || !lines):
			diag.E1124.AtControl(l.span(i, i+size), r).Report(l.bag)
		}
		i += size
	}
}

// runeEnd is the end of the rune at i, a lone invalid byte counting as one.
func (l *lexer) runeEnd(i int) int {
	_, size := utf8.DecodeRune(l.src[i:])
	return i + max(size, 1)
}
