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

func (l *lexer) illegal() {
	start := l.pos
	l.pos++
	diag.E1106.At(l.span(start, l.pos), l.text(start, l.pos)).Report(l.bag)
	l.emit(TokIllegal, start)
}

func (l *lexer) space() {
	start := l.pos
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		l.pos++
	}
	l.trivia(TriviaSpace, start)
}

// carriageReturn is a "\r" left by the "\r\n" normalization: E1124, kept as whitespace.
func (l *lexer) carriageReturn() {
	start := l.pos
	l.pos++
	diag.E1124.AtCr(l.span(start, l.pos)).Report(l.bag)
	l.trivia(TriviaSpace, start)
}

func (l *lexer) newline() {
	if l.inInterp() {
		l.interpNewline()
		return
	}
	start := l.pos
	l.pos++
	l.trivia(TriviaNewline, start)
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
	end := bytes.IndexByte(l.src[start:], '\n')
	if end < 0 {
		end = len(l.src) - start
	}
	l.pos = start + end
	l.checkText(start, l.pos, false)
	kind := TriviaLineComment
	if hasPrefixAt(l.src, start, docPrefix) && !hasPrefixAt(l.src, start, ordinaryDoc) {
		if l.lineText {
			diag.W1001.At(l.span(start, l.pos)).Report(l.bag)
		} else {
			kind = TriviaDocComment
		}
	}
	l.trivia(kind, start)
}

func (l *lexer) blockComment() {
	start := l.pos
	if l.inInterp() {
		diag.E1112.AtComment(l.span(start, start+len(blockOpen))).Report(l.bag)
	}
	end := bytes.Index(l.src[start+len(blockOpen):], []byte(blockClose))
	if end < 0 {
		diag.E1108.At(l.span(start, start+len(blockOpen))).Report(l.bag)
		l.pos = len(l.src)
	} else {
		l.pos = start + len(blockOpen) + end + len(blockClose)
	}
	l.checkText(start, l.pos, true)
	l.trivia(TriviaBlockComment, start)
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
