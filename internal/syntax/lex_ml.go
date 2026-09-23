package syntax

import (
	"bytes"

	"github.com/fantasim/canonlang/internal/diag"
)

// mlString is the layout state of a multiline string: the start of each line after the opening
// one, the last being the closing line.
type mlString struct {
	lines []int
}

// mlString reads the opening `"""` (raw after "r") and the rest of its line, which must be
// blank (E1122), then the content.
func (l *lexer) mlString(start int, raw bool) {
	l.pos = start + len(tripleQuote)
	if raw {
		l.pos = start + len(rawPrefix) + len(tripleQuote)
	}
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		l.pos++
	}
	ml := &mlString{}
	switch {
	case l.pos < len(l.src) && l.src[l.pos] == '\n':
		l.pos++
		ml.lines = append(ml.lines, l.pos)
	case l.pos < len(l.src):
		diag.E1122.AtOpen(l.span(l.pos, l.runeEnd(l.pos))).Report(l.bag)
	}
	if raw {
		l.rawMLText(start, ml)
		return
	}
	l.stringText(start, true, ml)
}

// closeML checks the closing `"""` at pos, then the layout of every content line, and moves
// past it.
func (l *lexer) closeML(ml *mlString) {
	at := l.pos
	l.pos += len(tripleQuote)
	lineStart := -1
	if n := len(ml.lines); n > 0 {
		lineStart = ml.lines[n-1]
	}
	if lineStart < 0 || !isBlank(l.src[lineStart:at]) {
		diag.E1122.AtClose(l.span(at, l.pos)).Report(l.bag)
		return
	}
	prefix := l.src[lineStart:at]
	if bytes.ContainsRune(prefix, ' ') && bytes.ContainsRune(prefix, '\t') {
		diag.E1102.At(l.span(lineStart, at)).Report(l.bag)
	}
	content := ml.lines[:len(ml.lines)-1]
	for i, ls := range content {
		end := lineStart - 1
		if i+1 < len(content) {
			end = content[i+1] - 1
		}
		if !indented(l.src[ls:end], prefix) {
			diag.E1122.AtIndent(l.span(ls, end)).Report(l.bag)
		}
	}
}

// indented reports a content line that starts with the indentation prefix, or is blank and
// no longer than it.
func indented(line, prefix []byte) bool {
	return bytes.HasPrefix(line, prefix) || (isBlank(line) && len(line) <= len(prefix))
}

func isBlank(b []byte) bool {
	return len(bytes.Trim(b, blankChars)) == 0
}

// rawString reads r"…" (GRAMMAR.md §2.6): no escapes, no interpolation, no line break.
func (l *lexer) rawString() {
	start := l.pos
	if hasPrefixAt(l.src, start+len(rawPrefix), tripleQuote) {
		l.mlString(start, true)
		return
	}
	l.pos = start + len(rawPrefix) + len(quoteText)
	for l.pos < len(l.src) && l.src[l.pos] != '"' && l.src[l.pos] != '\n' {
		l.pos = l.runeEnd(l.pos)
	}
	l.checkText(start, l.pos, false)
	if l.pos < len(l.src) && l.src[l.pos] == '"' {
		l.pos++
	} else {
		diag.E1107.At(l.span(start, l.pos)).Report(l.bag)
	}
	l.emit(TokRaw, start)
}

// rawMLText reads the content of r""" … """ up to the first `"""`.
func (l *lexer) rawMLText(start int, ml *mlString) {
	for l.pos < len(l.src) && !hasPrefixAt(l.src, l.pos, tripleQuote) {
		end := l.runeEnd(l.pos)
		l.checkText(l.pos, end, true)
		if l.src[l.pos] == '\n' {
			ml.lines = append(ml.lines, end)
		}
		l.pos = end
	}
	if l.pos < len(l.src) {
		l.closeML(ml)
	} else {
		diag.E1107.At(l.span(start, start+len(rawPrefix))).Report(l.bag)
	}
	l.emit(TokRawML, start)
}
