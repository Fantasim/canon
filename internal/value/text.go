package value

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/types"
)

// piece is one part of a composite's text form: a literal, or a component (v non-nil).
type piece struct {
	lit string
	v   Value
}

// layouter is a composite whose text form is its pieces, in order (STD-06).
type layouter interface {
	layout() []piece
}

// render is the text form of a composite (STD-06).
func render(v layouter) string {
	var b strings.Builder
	walkText(v, func(s string) bool {
		b.WriteString(s)
		return true
	})
	return b.String()
}

// walkText hands the text of v to emit piece by piece, in order, on an explicit stack
// (DECISIONS 197); it stops when emit returns false.
func walkText(v layouter, emit func(s string) bool) {
	stack := pushPieces(nil, v.layout())
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		text := p.lit
		switch x := p.v.(type) {
		case nil:
		case *Str:
			text = types.QuoteString(x.V)
		case layouter:
			stack = pushPieces(stack, x.layout())
			continue
		default:
			text = x.CanonText()
		}
		if !emit(text) {
			return
		}
	}
}

// pushPieces pushes ps last first, so they are written in order.
func pushPieces(stack, ps []piece) []piece {
	for _, p := range slices.Backward(ps) {
		stack = append(stack, p)
	}
	return stack
}

// enclosed is open, the components separated, then close.
func enclosed(open, closing string, vs []Value) []piece {
	out := make([]piece, 0, len(vs)*pieceStride+1)
	out = append(out, piece{lit: open})
	for i, v := range vs {
		if i > 0 {
			out = append(out, piece{lit: textSep})
		}
		out = append(out, piece{v: v})
	}
	return append(out, piece{lit: closing})
}

// TextLenUpTo is the length in bytes of v's text form, counted without building it and
// stopping at limit (DECISIONS 197).
func TextLenUpTo(v Value, limit int) int {
	top, ok := v.(layouter)
	if !ok {
		return min(len(v.CanonText()), limit)
	}
	n := 0
	walkText(top, func(s string) bool {
		n += len(s)
		return n < limit
	})
	return min(n, limit)
}

// TextUpTo is v's text form cut to its first limit bytes, at a character boundary; the walk
// stops there (DECISIONS 197).
func TextUpTo(v Value, limit int) string {
	top, ok := v.(layouter)
	if !ok {
		return cut(v.CanonText(), limit)
	}
	var b strings.Builder
	walkText(top, func(s string) bool {
		b.WriteString(s)
		return b.Len() < limit
	})
	return cut(b.String(), limit)
}

// cut is s's first limit bytes, backed off to the start of the character the cut falls in.
func cut(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	i := max(limit, 0)
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return s[:i]
}
