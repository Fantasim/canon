package fnsize

import (
	"fmt"
	"slices"
	"strings"
)

// kind is the class of a token; comments and spaces are not tokens.
type kind uint8

// token is one lexeme and the line it starts on (lines count from 1).
type token struct {
	kind kind
	text string
	line int
}

// handler lexes the token, comment or space that starts at the lexer's position; it always
// advances, or sets the error.
type handler func(l *lexer)

// language is what differs between the two tokenizers: a handler per leading byte, and whether
// the C++ rules hold (a quote inside a number is a digit separator, a backslash continues a line).
type language struct {
	table [byteRange]handler
	cpp   bool
}

// byteClass assigns a handler to every byte of a string.
type byteClass struct {
	bytes string
	run   handler
}

// newLanguage builds the dispatch table: spaces, identifiers, digits and punctuation for both
// languages, then the classes of the language on top.
func newLanguage(cpp bool, classes []byteClass) *language {
	lang := &language{cpp: cpp}
	for b := range lang.table {
		lang.table[b] = lexPunct
		switch {
		case b >= asciiLimit:
			lang.table[b] = lexIdent
		case isDigit(byte(b)):
			lang.table[b] = lexNumber
		}
	}
	common := []byteClass{
		{spaceBytes, lexSpace},
		{identStartBytes, lexIdent},
		{doubleQuote, lexDoubleQuote},
	}
	for _, c := range append(common, classes...) {
		for i := range len(c.bytes) {
			lang.table[c.bytes[i]] = c.run
		}
	}
	return lang
}

// lexer walks a source once; tokens are appended in source order.
type lexer struct {
	src  string
	pos  int
	line int
	toks []token
	err  error
	lang *language
}

// lex tokenizes src with the language's table.
func lex(src string, lang *language) ([]token, error) {
	l := &lexer{src: src, line: 1, lang: lang}
	for l.pos < len(l.src) && l.err == nil {
		l.step()
	}
	return l.toks, l.err
}

// step runs the handler of the byte at the position.
func (l *lexer) step() { l.lang.table[l.src[l.pos]](l) }

// emit appends the token src[start:l.pos] that began on line.
func (l *lexer) emit(k kind, start, line int) {
	l.toks = append(l.toks, token{kind: k, text: l.src[start:l.pos], line: line})
}

// skipTo moves to end, counting the newlines it passes.
func (l *lexer) skipTo(end int) {
	l.line += strings.Count(l.src[l.pos:end], "\n")
	l.pos = end
}

// fail records an unterminated construct that began on line.
func (l *lexer) fail(what string, line int) {
	l.err = fmt.Errorf("%s at line %d: %w", what, line, errUnterminated)
	l.pos = len(l.src)
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func lexSpace(l *lexer) {
	end := l.pos
	for end < len(l.src) && strings.IndexByte(spaceBytes, l.src[end]) >= 0 {
		end++
	}
	l.skipTo(end)
}

func isIdentByte(b byte) bool {
	return strings.IndexByte(identStartBytes, b) >= 0 || isDigit(b) || b >= asciiLimit
}

// scanIdent returns the end of the identifier that starts at the position.
func (l *lexer) scanIdent() int {
	end := l.pos
	for end < len(l.src) && isIdentByte(l.src[end]) {
		end++
	}
	return end
}

func lexIdent(l *lexer) {
	start := l.pos
	l.pos = l.scanIdent()
	l.emit(kindIdent, start, l.line)
}

func lexNumber(l *lexer) {
	start := l.pos
	end := l.pos + 1
	for end < len(l.src) && l.numberContinues(end) {
		end++
	}
	l.pos = end
	l.emit(kindNumber, start, l.line)
}

// numberContinues reports whether the byte at i belongs to the number that began before it: a
// preprocessing number, with the digit separator of C++ and the signed exponent.
func (l *lexer) numberContinues(i int) bool {
	b := l.src[i]
	switch {
	case isIdentByte(b), b == '.':
		return true
	case b == '\'':
		return l.lang.cpp && i+1 < len(l.src) && isIdentByte(l.src[i+1])
	case b == '+', b == '-':
		return strings.IndexByte(exponentBytes, l.src[i-1]) >= 0
	}
	return false
}

// lexPunct reads the longest punctuator at the position.
func lexPunct(l *lexer) {
	start := l.pos
	l.pos++
	for _, p := range punctuators {
		if strings.HasPrefix(l.src[start:], p) {
			l.pos = start + len(p)
			break
		}
	}
	l.emit(kindPunct, start, l.line)
}

// lexDot reads a number that starts with a point, or punctuation.
func lexDot(l *lexer) {
	if l.pos+1 < len(l.src) && isDigit(l.src[l.pos+1]) {
		lexNumber(l)
		return
	}
	lexPunct(l)
}

// scanQuoted reads a quoted literal whose opening quote is at the position, after prefix bytes
// already consumed; a backslash escapes the next byte, and a newline ends nothing but an error.
func (l *lexer) scanQuoted(start int, quote byte) {
	line := l.line
	i := l.pos + 1
	for i < len(l.src) {
		switch l.src[i] {
		case quote:
			l.pos = i + 1
			l.emit(kindString, start, line)
			return
		case '\\':
			i++
			if i < len(l.src) && l.src[i] == '\n' {
				l.line++
			}
		case '\n':
			l.fail(whatQuoted, line)
			return
		}
		i++
	}
	l.fail(whatQuoted, line)
}

func lexDoubleQuote(l *lexer) { l.scanQuoted(l.pos, '"') }

func lexSingleQuote(l *lexer) { l.scanQuoted(l.pos, '\'') }

// lineEnd returns the index of the newline that ends the line holding from; in C++ a backslash
// before the newline continues the line.
func (l *lexer) lineEnd(from int) int {
	end := from
	for end < len(l.src) && (l.src[end] != '\n' || l.lang.cpp && end > 0 && l.src[end-1] == '\\') {
		end++
	}
	return end
}

// lexLineComment skips to the end of the line.
func lexLineComment(l *lexer) { l.skipTo(l.lineEnd(l.pos)) }

// lexBlockComment skips a comment that opens at the position.
func lexBlockComment(l *lexer) {
	end := strings.Index(l.src[l.pos+len(blockOpen):], blockClose)
	if end < 0 {
		l.fail(whatComment, l.line)
		return
	}
	l.skipTo(l.pos + len(blockOpen) + end + len(blockClose))
}

// commentAhead skips a line or block comment at the position and reports whether there was one.
func commentAhead(l *lexer) bool {
	switch {
	case strings.HasPrefix(l.src[l.pos:], lineOpen):
		lexLineComment(l)
	case strings.HasPrefix(l.src[l.pos:], blockOpen):
		lexBlockComment(l)
	default:
		return false
	}
	return true
}

// matchBrackets pairs every (, [ and { of toks with its closer, both ways; -1 marks the rest.
func matchBrackets(toks []token) ([]int, error) {
	match := make([]int, len(toks))
	var open []int
	for i, t := range toks {
		match[i] = -1
		if t.kind != kindPunct {
			continue
		}
		if slices.Contains(openers, t.text) {
			open = append(open, i)
			continue
		}
		want, closing := openerOf[t.text]
		if !closing {
			continue
		}
		if len(open) == 0 || toks[open[len(open)-1]].text != want {
			return nil, unbalanced(t)
		}
		top := open[len(open)-1]
		match[i], match[top] = top, i
		open = open[:len(open)-1]
	}
	if len(open) > 0 {
		return nil, unbalanced(toks[open[len(open)-1]])
	}
	return match, nil
}

func unbalanced(t token) error { return fmt.Errorf("%q at line %d: %w", t.text, t.line, errUnbalanced) }
