package fnsize

import (
	"slices"
	"strings"
)

// cppLanguage reads C++17: comments, character and raw strings, preprocessor lines and digit
// separators are skipped or kept whole.
var cppLanguage = newLanguage(true, []byteClass{
	{identStartBytes, lexCppIdent},
	{"/", lexCppSlash},
	{"#", lexDirective},
	{"'", lexSingleQuote},
	{punctDot, lexDot},
})

func lexCppSlash(l *lexer) {
	if !commentAhead(l) {
		lexPunct(l)
	}
}

// lexDirective skips a preprocessor line (and its continuations) when "#" opens a line; a "#"
// elsewhere is punctuation.
func lexDirective(l *lexer) {
	for i := l.pos - 1; i >= 0 && l.src[i] != '\n'; i-- {
		if strings.IndexByte(blankBytes, l.src[i]) < 0 {
			lexPunct(l)
			return
		}
	}
	l.skipTo(l.lineEnd(l.pos))
}

// lexCppIdent reads an identifier, or a literal when the identifier is an encoding prefix
// ("L", "u8", "R", "LR", ...) glued to its quote.
func lexCppIdent(l *lexer) {
	start := l.pos
	end := l.scanIdent()
	l.pos = end
	if end < len(l.src) && slices.Contains(stringPrefixes, l.src[start:end]) {
		switch {
		case l.src[end] == '"' && strings.HasSuffix(l.src[start:end], rawMark):
			l.scanRaw(start)
			return
		case l.src[end] == '"' || l.src[end] == '\'':
			l.scanQuoted(start, l.src[end])
			return
		}
	}
	l.emit(kindIdent, start, l.line)
}

// scanRaw reads R"delim( ... )delim" whose opening quote is at the position.
func (l *lexer) scanRaw(start int) {
	line := l.line
	open := strings.IndexByte(l.src[l.pos:], '(')
	if open < 0 || open-1 > maxRawDelim {
		l.fail(whatRaw, line)
		return
	}
	closing := punctCloseParen + l.src[l.pos+1:l.pos+open] + doubleQuote
	end := strings.Index(l.src[l.pos+open:], closing)
	if end < 0 {
		l.fail(whatRaw, line)
		return
	}
	l.skipTo(l.pos + open + end + len(closing))
	l.emit(kindString, start, line)
}
