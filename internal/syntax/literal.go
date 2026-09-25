package syntax

import (
	"bytes"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// intValue is the exact value of an INT token (GRAMMAR.md §2.4, LEX-05).
func intValue(text string) *big.Int {
	base, digits := decimalBase, text
	if b, d, ok := prefixed(text); ok {
		base, digits = b, d
	}
	v, ok := new(big.Int).SetString(strings.ReplaceAll(digits, digitSep, ""), base)
	if !ok {
		return new(big.Int)
	}
	return v
}

// floatParts is the exact value of a FLOAT token as coef × 10^exp; ok is false when the
// exponent does not fit an int64.
func floatParts(text string) (*big.Int, int64, bool) {
	mantissa, exp := text, ""
	if i := strings.IndexAny(text, expLetters); i >= 0 {
		mantissa, exp = text[:i], text[i:]
	}
	whole, frac, _ := strings.Cut(strings.ReplaceAll(mantissa, digitSep, ""), fractionDot)
	coef, ok := new(big.Int).SetString(whole+frac, decimalBase)
	if !ok {
		return new(big.Int), 0, false
	}
	e := big.NewInt(-int64(len(frac)))
	if exp != "" {
		x, ok := exponent(exp)
		if !ok {
			return coef, 0, false
		}
		e.Add(e, x)
	}
	return coef, e.Int64(), e.IsInt64()
}

// durationMillis is the magnitude of a DURATION token in milliseconds (GRAMMAR.md §2.5).
func durationMillis(text string) uint64 { return classifyDuration(text).millis }

// parseSpec reads a format spec, `\+?,?(\.[0-9]{1,2})?`, non-empty, N ≤ 20 (GRAMMAR.md §2.6).
func parseSpec(spec []byte) (FormatSpec, bool) {
	fs := FormatSpec{Decimals: NoDecimals}
	s := string(spec)
	fs.Plus = strings.HasPrefix(s, specPlus)
	s = strings.TrimPrefix(s, specPlus)
	fs.Comma = strings.HasPrefix(s, specComma)
	s = strings.TrimPrefix(s, specComma)
	if rest, ok := strings.CutPrefix(s, fractionDot); ok {
		n, err := strconv.Atoi(rest)
		if err != nil || rest == "" || len(rest) > maxSpecDigits || !isDigits([]byte(rest)) || n > maxDecimals {
			return fs, false
		}
		fs.Decimals, s = n, ""
	}
	return fs, s == "" && len(spec) > 0
}

// validScalar reports 1 to 6 hex digits naming a Unicode scalar value.
func validScalar(hex []byte) bool {
	if len(hex) == 0 || len(hex) > maxHexDigits {
		return false
	}
	_, ok := scalar(hex)
	return ok
}

// scalar is the Unicode scalar value hex names, when it names one.
func scalar(hex []byte) (rune, bool) {
	v, err := strconv.ParseInt(string(hex), hexBase, runeBits)
	if err != nil || v < 0 || v > unicode.MaxRune {
		return utf8.RuneError, false
	}
	r := rune(v)
	return r, utf8.ValidRune(r)
}

// regexPattern is a regex body with each unescaped `\/` replaced by "/" (GRAMMAR.md §2.7, LEX-01).
func regexPattern(body []byte) string {
	var b strings.Builder
	b.Grow(len(body))
	for i := 0; i < len(body); {
		if body[i] == '\\' && i+1 < len(body) {
			if body[i+1] == '/' {
				b.WriteByte('/')
			} else {
				b.Write(body[i : i+pairWidth])
			}
			i += pairWidth
			continue
		}
		b.WriteByte(body[i])
		i++
	}
	return b.String()
}

// unescape decodes the text of a string piece: escapes, "{{" and "}}"; an escape the lexer
// refused is kept as written.
func unescape(raw []byte) string {
	var b strings.Builder
	for i := 0; i < len(raw); {
		c := raw[i]
		switch {
		case c == '\\' && i+1 < len(raw):
			i += writeEscape(&b, raw[i:])
		case (c == '{' || c == '}') && i+1 < len(raw) && raw[i+1] == c:
			b.WriteByte(c)
			i += pairWidth
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// writeEscape writes the escape at the start of raw and returns its length.
func writeEscape(b *strings.Builder, raw []byte) int {
	if r, ok := simpleEscapes[raw[1]]; ok {
		b.WriteByte(r)
		return pairWidth
	}
	end := bytes.IndexByte(raw, '}')
	if raw[1] == 'u' && len(raw) > unicodeOpen && raw[unicodeOpen-1] == '{' && end >= unicodeOpen {
		if r, ok := scalar(raw[unicodeOpen:end]); ok && end-unicodeOpen <= maxHexDigits {
			b.WriteRune(r)
			return end + 1
		}
	}
	b.WriteByte(raw[0])
	return 1
}

// docText is the normalized text of a doc block (LEX-07): each line without "///" and one space,
// trailing blanks removed, joined by "\n", outer empty lines dropped.
func docText(lines []string) string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimPrefix(strings.TrimPrefix(l, docPrefix), spaceText)
		out = append(out, strings.TrimRight(l, blankChars))
	}
	for len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, lf)
}
