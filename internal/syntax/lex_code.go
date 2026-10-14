package syntax

import (
	"bytes"
	"regexp"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
)

// codeTable dispatches on the first byte of a token or trivia in code and interpolations;
// punctByFirst lists the punctuation starting with each byte, longest first.
var (
	codeTable    [utf8Self]func(*lexer)
	punctByFirst [utf8Self][]TokenKind
	keywords     = map[string]TokenKind{}
)

func init() {
	for c := range codeTable {
		codeTable[c] = (*lexer).illegal
	}
	for k := TokEllipsis; k <= TokHash; k++ {
		if k != TokUnderscore {
			first := tokenNames[k][0]
			punctByFirst[first] = append(punctByFirst[first], k)
			codeTable[first] = (*lexer).punct
		}
	}
	for _, ps := range punctByFirst {
		slices.SortStableFunc(ps, func(a, b TokenKind) int { return len(tokenNames[b]) - len(tokenNames[a]) })
	}
	for k := KwAnd; k < TokenKindCount; k++ {
		keywords[tokenNames[k]] = k
	}
	for c := byte('a'); c <= 'z'; c++ {
		codeTable[c], codeTable[c-'a'+'A'] = (*lexer).ident, (*lexer).ident
	}
	for c := byte('0'); c <= '9'; c++ {
		codeTable[c] = (*lexer).number
	}
	codeTable['_'], codeTable['r'] = (*lexer).ident, (*lexer).rawOrIdent
	codeTable[' '], codeTable['\t'] = (*lexer).space, (*lexer).space
	codeTable['\r'], codeTable['\n'] = (*lexer).carriageReturn, (*lexer).newline
	codeTable['"'], codeTable['/'] = (*lexer).quote, (*lexer).slash
	codeTable[backtick] = (*lexer).quotedWord
	codeTable['#'], codeTable['}'], codeTable[':'] = (*lexer).hash, (*lexer).rbrace, (*lexer).colon
}

// LookupWord is the kind of a word: its reserved word, TokUnderscore for "_", else TokIdent.
func LookupWord(word string) TokenKind {
	if k, ok := keywords[word]; ok {
		return k
	}
	if word == Blank {
		return TokUnderscore
	}
	return TokIdent
}

// IsNameable reports a reserved word read as a name in value position (GRAMMAR.md §4.3 (c)).
func IsNameable(k TokenKind) bool { return k < TokenKindCount && nameable[k] }

// IsQuotable reports a reserved word that a quoted word may spell (GRAMMAR.md §2.3).
func IsQuotable(k TokenKind) bool { return k < TokenKindCount && quotable[k] }

// ValueText is the value-position text of a data name (GRAMMAR.md §4.3, §2.3).
func ValueText(word string) (string, bool) {
	if !isWordText(word) {
		return "", false
	}
	switch k := LookupWord(word); {
	case k == TokIdent || IsNameable(k):
		return word, true
	case IsQuotable(k):
		return quoteWord(word), true
	default:
		return "", false
	}
}

// isWordText reports letter-or-underscore then word bytes (GRAMMAR.md §2.3).
func isWordText(s string) bool {
	return s != "" && !isDigit(s[0]) && !slices.ContainsFunc([]byte(s), func(c byte) bool { return !isWordByte(c) })
}

func quoteWord(word string) string { return string(backtick) + word + string(backtick) }

// quotedWord reads a QWORD (GRAMMAR.md §2.3, §2.9).
func (l *lexer) quotedWord() {
	start, end := l.pos, l.pos+1
	if end < len(l.src) && !isDigit(l.src[end]) {
		for end < len(l.src) && isWordByte(l.src[end]) {
			end++
		}
	}
	if end == start+1 || end >= len(l.src) || l.src[end] != backtick {
		l.illegal()
		return
	}
	l.pos = end + 1
	if word := l.text(start+1, end); !IsQuotable(LookupWord(word)) {
		diag.E1138.At(l.span(start, l.pos), word).Report(l.bag)
	}
	l.emit(TokQWord, start)
}

func (l *lexer) illegal() {
	start := l.pos
	l.pos++
	diag.E1106.At(l.span(start, l.pos), l.text(start, l.pos)).Report(l.bag)
	l.emit(TokIllegal, start)
}

func (l *lexer) space() { l.pos = spaceEnd(l.src, l.pos) }

// carriageReturn is a "\r" left by the "\r\n" normalization: E1124, kept as whitespace.
func (l *lexer) carriageReturn() {
	start := l.pos
	l.pos++
	diag.E1124.AtCr(l.span(start, l.pos)).Report(l.bag)
}

func (l *lexer) newline() {
	if l.inInterp() {
		l.interpNewline()
		return
	}
	l.pos++
	l.lineText = false
}

// punct emits the longest punctuation at pos (GRAMMAR.md §2.8) and tracks interpolation depth.
func (l *lexer) punct() {
	start := l.pos
	for _, k := range punctByFirst[l.src[l.pos]] {
		if hasPrefixAt(l.src, l.pos, tokenNames[k]) {
			l.pos += len(tokenNames[k])
			l.emit(k, start)
			l.track(k)
			return
		}
	}
	l.illegal()
}

func (l *lexer) track(k TokenKind) {
	if !l.inInterp() {
		return
	}
	switch k {
	case TokLParen, TokLBrack, TokLBrace:
		l.top().depth++
	case TokRParen, TokRBrack, TokRBrace:
		l.top().depth = max(l.top().depth-1, 0)
	default:
	}
}

func (l *lexer) ident() {
	start := l.pos
	for l.pos < len(l.src) && isWordByte(l.src[l.pos]) {
		l.pos++
	}
	l.emit(LookupWord(l.text(start, l.pos)), start)
}

func (l *lexer) rawOrIdent() {
	if l.peek(1) == '"' {
		l.rawString()
		return
	}
	l.ident()
}

func (l *lexer) quote() {
	if hasPrefixAt(l.src, l.pos, tripleQuote) {
		l.mlString(l.pos, false)
		return
	}
	l.pos++
	l.stringText(openString(l.pos-1, nil))
}

// hash is "#": a token directly after "[" in an amend path (GRAMMAR.md §5.7), E1106 elsewhere.
func (l *lexer) hash() {
	prev := l.toks[len(l.toks)-1]
	if prev.Kind != TokLBrack || int(prev.End) != l.pos {
		l.illegal()
		return
	}
	l.punct()
}

// slash starts a comment, a regular expression after "(" or ",", or a division (GRAMMAR.md §2.7).
func (l *lexer) slash() {
	switch next := l.peek(1); {
	case next == '/':
		l.lineComment()
	case next == '*':
		l.blockComment()
	case l.prevKind() == TokLParen || l.prevKind() == TokComma:
		l.regex()
	default:
		l.punct()
	}
}

func (l *lexer) lineComment() {
	start := l.pos
	if l.inInterp() {
		diag.E1112.AtComment(l.span(start, start+len(lineCommentText))).Report(l.bag)
	}
	l.pos = lineCommentEnd(l.src, start)
	l.checkText(start, l.pos, false)
	if l.lineText && isDocLine(l.src, start) {
		diag.W1001.At(l.span(start, l.pos)).Report(l.bag)
	}
}

func (l *lexer) blockComment() {
	start := l.pos
	if l.inInterp() {
		diag.E1112.AtComment(l.span(start, start+len(blockOpen))).Report(l.bag)
	}
	end, closed := blockCommentEnd(l.src, start)
	if !closed {
		diag.E1108.At(l.span(start, start+len(blockOpen))).Report(l.bag)
	}
	l.pos = end
	l.checkText(start, l.pos, true)
	l.lineText = true
}

// regex reads "/body/" (GRAMMAR.md §2.7): "\" escapes the next character, "[…]" hides "/".
func (l *lexer) regex() {
	start := l.pos
	l.pos++
	class := false
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		c := l.src[l.pos]
		switch {
		case c == '\\' && l.peek(1) != '\n' && l.pos+1 < len(l.src):
			l.pos = l.runeEnd(l.pos + 1)
			continue
		case c == '/' && !class:
			l.pos++
			l.endRegex(start, true)
			return
		case c == '[' || c == ']':
			class = c == '['
		}
		l.pos = l.runeEnd(l.pos)
	}
	l.endRegex(start, false)
}

func (l *lexer) endRegex(start int, closed bool) {
	l.checkText(start, l.pos, false)
	l.emit(TokRegex, start)
	if !closed {
		diag.E1113.At(l.span(start, l.pos)).Report(l.bag)
		return
	}
	if _, err := regexp.Compile(regexPattern(l.src[start+1 : l.pos-1])); err != nil {
		diag.E1114.At(l.span(start, l.pos), err.Error()).Report(l.bag)
	}
}

func hasPrefixAt(src []byte, i int, prefix string) bool {
	return i <= len(src) && bytes.HasPrefix(src[i:], []byte(prefix))
}

func isWordByte(c byte) bool {
	return c == '_' || isDigit(c) || (c|asciiLowerBit >= 'a' && c|asciiLowerBit <= 'z')
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
