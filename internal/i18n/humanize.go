package i18n

import (
	"strings"
	"unicode/utf8"
)

// Humanize is a field, method or value's default label when its view gives none, e.g.
// "farmPurchasePrice" -> "Farm purchase price" (VIEWMODEL.md X1).
func Humanize(name string) string {
	spaced := spaceBoundaries(name)
	spaced = strings.ReplaceAll(spaced, underscorePrefix, spaceText)
	spaced = strings.Join(strings.Fields(spaced), spaceText)
	spaced = strings.ToLower(spaced)
	return upperFirst(spaced)
}

// spaceBoundaries inserts one space at each lower/upper ASCII boundary of name.
func spaceBoundaries(name string) string {
	var b strings.Builder
	for i, r := range name {
		if i > 0 && isBoundary(rune(name[i-1]), r) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isBoundary reports a lower-case letter or digit followed by an upper-case ASCII letter.
func isBoundary(prev, cur rune) bool {
	lower := prev >= 'a' && prev <= 'z' || prev >= '0' && prev <= '9'
	upper := cur >= 'A' && cur <= 'Z'
	return lower && upper
}

// upperFirst upper-cases s's first rune (X1); s is lower-case already.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return strings.ToUpper(string(r)) + s[size:]
}
