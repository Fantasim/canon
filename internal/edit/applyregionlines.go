package edit

import (
	"bytes"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// span is n's bytes, its leading comments and the comment ending its last line included.
func (t *canonTree) span(n syntax.Node) region {
	first, last := t.f.Tokens[n.First()], t.f.Tokens[n.Last()]
	r := region{int(first.Start), int(last.End), regionGone}
	for _, tr := range first.Leading {
		if comment(tr) {
			r.lo = int(tr.Start)
			break
		}
	}
	for _, tr := range last.Trailing {
		if comment(tr) {
			r.hi = int(tr.End)
		}
	}
	return r
}

// comment reports tr a comment of any kind.
func comment(tr syntax.Trivia) bool {
	return tr.Kind == syntax.TriviaLineComment || tr.Kind == syntax.TriviaBlockComment || tr.Kind == syntax.TriviaDocComment
}

// itemLines are the whole lines of r, and the blank line next to them a removal takes when it
// would leave two in a row, or one after an opening or before a closing bracket.
func (t *canonTree) itemLines(r region) region {
	src := t.f.Src.Content
	lo, hi := lineStart(src, r.lo), endOfLine(src, r.hi)
	prevBlank := lo > 1 && src[lo-1-1] == '\n'
	nextBlank := hi < len(src) && src[hi] == '\n'
	opens := lo == 0 || strings.ContainsAny(lastByte(src[:lo-1]), openBrackets)
	closes := hi == len(src) || strings.ContainsAny(firstByte(src[hi:]), closeBrackets)
	switch {
	case nextBlank && (prevBlank || opens):
		hi++
	case prevBlank && (closes || hi == len(src)):
		lo--
	}
	return region{lo, hi, regionGone}
}

// lineStart is the offset of the start of the line holding offset at.
func lineStart(src []byte, at int) int {
	return bytes.LastIndexByte(src[:at], '\n') + 1
}

// endOfLine is the offset past the line break ending the line holding offset at.
func endOfLine(src []byte, at int) int {
	if i := bytes.IndexByte(src[at:], '\n'); i >= 0 {
		return at + i + 1
	}
	return len(src)
}

// lastByte is the last byte of a line other than space, as a string; "" for none.
func lastByte(line []byte) string {
	t := bytes.TrimRight(line[bytes.LastIndexByte(line, '\n')+1:], spaceChars)
	if len(t) == 0 {
		return ""
	}
	return string(t[len(t)-1:])
}

// firstByte is the first byte other than space of the text's first line, as a string.
func firstByte(text []byte) string {
	t := bytes.TrimLeft(text, spaceChars)
	if len(t) == 0 {
		return ""
	}
	return string(t[:1])
}
