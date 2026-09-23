package types

import (
	"strconv"
	"strings"
)

// FloatText is the canonical text of a finite float of bits 64 or 32: ECMAScript
// Number::toString over the shortest round-tripping digits (STD-06).
func FloatText(x float64, bits int) string {
	if x == 0 {
		return textZero
	}
	sign := ""
	if x < 0 {
		sign, x = textMinus, -x
	}
	mant, exp, _ := strings.Cut(strconv.FormatFloat(x, 'e', -1, bits), textExp)
	e, _ := strconv.Atoi(exp)
	return sign + floatLayout(strings.Replace(mant, textDot, "", 1), e+1)
}

// floatLayout places the digits d of a value d × 10^(n−len(d)) as ECMAScript does.
func floatLayout(d string, n int) string {
	k := len(d)
	switch {
	case k <= n && n <= floatMaxExp10:
		return d + strings.Repeat(textZero, n-k)
	case 0 < n && n <= floatMaxExp10:
		return d[:n] + textDot + d[n:]
	case floatMinExp10 < n && n <= 0:
		return textPoint + strings.Repeat(textZero, -n) + d
	}
	e, sign := n-1, textPlus
	if e < 0 {
		e, sign = -e, textMinus
	}
	m := d[:1]
	if k > 1 {
		m += textDot + d[1:]
	}
	return m + textExp + sign + strconv.Itoa(e)
}

// DurationText is the canonical duration literal of ms milliseconds (STD-06): 1m30s, -250ms.
func DurationText(ms int64) string {
	if ms == 0 {
		return textZeroDur
	}
	var b strings.Builder
	if ms < 0 {
		b.WriteString(textMinus)
	}
	for _, unit := range durationParts {
		q := ms / unit.Millis()
		ms -= q * unit.Millis()
		if q != 0 {
			b.WriteString(strconv.FormatInt(max(q, -q), 10))
			b.WriteString(unit.String())
		}
	}
	return b.String()
}

// QuoteString writes s as a Canon string literal, escaped as the text form nests strings.
func QuoteString(s string) string {
	var b strings.Builder
	b.WriteString(textQuote)
	for i := range len(s) {
		c := s[i]
		switch {
		case quoteEscapes[c] != "":
			b.WriteString(quoteEscapes[c])
		case c < firstPrintable || c == deleteByte:
			b.WriteString(textEscapeOpen + strings.ToUpper(strconv.FormatUint(uint64(c), 16)) + textCloseMap)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString(textQuote)
	return b.String()
}
