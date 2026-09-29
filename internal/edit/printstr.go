package edit

import (
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// canonQuote is s as a plain Canon string literal whose text is s: escaped as QuoteString
// escapes, and each brace as `\{` or `\}`, which no interpolation reads.
func canonQuote(s string) string {
	var b strings.Builder
	b.WriteString(quoteMark)
	for {
		i := strings.IndexAny(s, braceChars)
		if i < 0 {
			b.WriteString(quotedInner(s))
			break
		}
		b.WriteString(quotedInner(s[:i]))
		b.WriteString(escapeMark + s[i:i+1])
		s = s[i+1:]
	}
	b.WriteString(quoteMark)
	return b.String()
}

// quotedInner is s as QuoteString writes it, without its quotes.
func quotedInner(s string) string {
	q := types.QuoteString(s)
	return q[len(quoteMark) : len(q)-len(quoteMark)]
}
