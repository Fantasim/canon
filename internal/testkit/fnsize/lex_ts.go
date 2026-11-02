package fnsize

import "slices"

// tsLanguage reads TypeScript: comments, quoted strings, regular expressions and template
// literals (with their ${} expressions, nested) are skipped or kept whole.
var tsLanguage = newLanguage(false, []byteClass{
	{"/", lexTSSlash},
	{"'", lexSingleQuote},
	{"`", lexTemplate},
	{punctDot, lexDot},
})

// lexTSSlash reads a comment, a regular expression or a division sign.
func lexTSSlash(l *lexer) {
	if commentAhead(l) || (l.regexAllowed() && l.scanRegex()) {
		return
	}
	lexPunct(l)
}

// regexAllowed reports whether a "/" at the position starts a regular expression, by the token
// before it.
func (l *lexer) regexAllowed() bool {
	if len(l.toks) == 0 {
		return true
	}
	prev := l.toks[len(l.toks)-1]
	switch prev.kind {
	case kindPunct:
		return !slices.Contains(divisionLeaders, prev.text)
	case kindIdent:
		return slices.Contains(regexLeaders, prev.text)
	case kindNumber, kindString:
		return false
	}
	return false
}

// scanRegex reads /body/flags on one line; false (nothing consumed) when it does not close.
func (l *lexer) scanRegex() bool {
	inClass := false
	for i := l.pos + 1; i < len(l.src) && l.src[i] != '\n'; i++ {
		switch {
		case l.src[i] == '\\':
			i++
		case l.src[i] == '[':
			inClass = true
		case l.src[i] == ']':
			inClass = false
		case l.src[i] == '/' && !inClass:
			start := l.pos
			l.pos = i + 1
			for l.pos < len(l.src) && isIdentByte(l.src[l.pos]) {
				l.pos++
			}
			l.emit(kindString, start, l.line)
			return true
		}
	}
	return false
}

// lexTemplate reads a template literal; a ${ opens an expression that is lexed like code.
func lexTemplate(l *lexer) {
	start, line := l.pos, l.line
	for i := l.pos + 1; i < len(l.src); i++ {
		switch {
		case l.src[i] == '`':
			l.pos = i + 1
			l.emit(kindString, start, line)
			return
		case l.src[i] == '\\':
			i++
			l.countNewline(i)
		case l.src[i] == '\n':
			l.line++
		case l.src[i] == '$' && l.src[min(i+1, len(l.src)-1)] == '{':
			l.pos = i + len(exprOpen)
			if !l.templateExpr() {
				return
			}
			i = l.pos - 1
		}
	}
	l.fail(whatTemplate, line)
}

// countNewline counts the newline at i, if there is one.
func (l *lexer) countNewline(i int) {
	if i < len(l.src) && l.src[i] == '\n' {
		l.line++
	}
}

// templateExpr lexes the expression of a ${ up to its closing brace, keeping its tokens out of
// the stream; false when the source ends first.
func (l *lexer) templateExpr() bool {
	saved := l.toks
	l.toks = nil
	depth := 0
	for l.pos < len(l.src) && l.err == nil {
		before := len(l.toks)
		l.step()
		if len(l.toks) == before {
			continue
		}
		if closes := braceDelta(l.toks[len(l.toks)-1], &depth); closes {
			l.toks = saved
			return true
		}
	}
	l.toks = saved
	if l.err == nil {
		l.fail(whatExpr, l.line)
	}
	return false
}

// braceDelta tracks the braces of an expression; true when t closes the one that opened it.
func braceDelta(t token, depth *int) bool {
	switch {
	case isPunct(t, punctOpenBrace):
		*depth++
	case isPunct(t, punctCloseBrace) && *depth == 0:
		return true
	case isPunct(t, punctCloseBrace):
		*depth--
	}
	return false
}
