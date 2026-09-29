package edit

import (
	"bytes"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
)

// span is n's bytes, its leading comments and the comment ending its last line included.
func (t *canonTree) span(n syntax.Node) region {
	first, last := t.f.Tokens[n.First()], t.f.Tokens[n.Last()]
	r := region{lo: int(first.Start), hi: int(last.End), kind: regionGone}
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

// owned is n's span with the comma after it and that comma's trailing comments: format moves
// and removes them with n (DECISIONS 216).
func (t *canonTree) owned(n syntax.Node) region {
	r := t.span(n)
	c, ok := t.comma(n)
	if !ok {
		return r
	}
	r.hi = max(r.hi, int(t.f.Tokens[c].End))
	for _, tr := range t.f.Tokens[c].Trailing {
		if comment(tr) {
			r.hi = int(tr.End)
		}
	}
	return r
}

// comma is the comma after item n, if the token after its last one is a comma.
func (t *canonTree) comma(n syntax.Node) (syntax.Tok, bool) {
	c := n.Last() + 1
	for int(c) < len(t.f.Tokens) && t.f.Tokens[c].Kind == syntax.TokNL {
		c++
	}
	return c, int(c) < len(t.f.Tokens) && t.f.Tokens[c].Kind == syntax.TokComma
}

// moveRegions are where a Move writes (log-2026-09-29 M4 U1b, U1b-r): its list printed again
// when on one line; else the item's lines gone with the blank line a removal takes, and at the
// insertion point after the item before its new place, exactly those lines, but for its comma.
func (t *canonTree) moveRegions(c format.Change) []region {
	// FORMATTER.md §13 steps 4 and 5, API.md M4, M6
	list := t.parent[c.Node]
	if t.relaid(list) {
		return []region{t.reprint(list)}
	}
	src := t.f.Src.Content
	at := endOfLine(src, int(t.f.Tokens[c.List].End))
	if items := t.items(list); c.At > 0 && c.At <= len(items) {
		at = endOfLine(src, t.owned(items[c.At-1]).hi)
	}
	own := t.owned(c.Node)
	moved := region{lo: at, hi: at, kind: regionMoved, fromLo: lineStart(src, own.lo), fromHi: endOfLine(src, own.hi)}
	moved.comma = int(t.f.Tokens[c.Node.Last()].End)
	return []region{t.itemLines(own), moved}
}

// neighbours are the comma regions of the kept items just before the places a change of a
// broken brace list leaves or fills: the comma after each may come or go as the item after it
// changes, a re-print of that item that API.md M5 requires (log-2026-09-29 M4 U1b-r).
func (t *canonTree) neighbours(c format.Change) []region {
	// DECISIONS 211, FORMATTER.md §13 steps 4 and 5
	list, points := t.changePoints(c)
	if list == nil || list == syntax.Node(t.f) || t.relaid(list) || t.f.Tokens[t.open(list)].Kind != syntax.TokLBrace {
		return nil
	}
	items := t.items(list)
	var out []region
	for _, p := range points {
		p--
		for p >= 0 && p < len(items) && t.leaving[items[p]] {
			p--
		}
		if p >= 0 && p < len(items) {
			out = append(out, t.commaRegion(items[p]))
		}
	}
	return out
}

// changePoints are the list a change touches and the positions among its items where an item
// leaves or arrives; no list for a change of no list.
func (t *canonTree) changePoints(c format.Change) (syntax.Node, []int) {
	switch {
	case c.Kind == format.Insert && c.List != syntax.NoTok:
		return t.around(c.List), []int{c.At}
	case c.Kind == format.Remove, c.Kind == format.Move:
		list := t.parent[c.Node]
		if list == nil {
			return nil, nil
		}
		from := slices.Index(t.items(list), c.Node)
		if c.Kind == format.Remove {
			return list, []int{from}
		}
		return list, []int{from, c.At}
	}
	return nil, nil
}

// commaRegion is where the comma right after item n's last token may come or go.
func (t *canonTree) commaRegion(n syntax.Node) region {
	lo := int(t.f.Tokens[n.Last()].End)
	r := region{lo: lo, hi: lo, kind: regionComma}
	if c, ok := t.comma(n); ok && int(t.f.Tokens[c].Start) == lo {
		r.hi = int(t.f.Tokens[c].End)
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
	return region{lo: lo, hi: hi, kind: regionGone}
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
