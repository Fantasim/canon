package tsgen

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// quote is s as a TypeScript string literal: JSON, without HTML escapes (CODEGEN.md §2.6).
func quote(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return strconv.Quote(s)
	}
	return strings.TrimSuffix(b.String(), newline)
}

// docComment is doc text as a TypeScript doc comment at indent (CODEGEN.md §2.6): one line, or several with ` * ` prefixes; `*/` is written `*\/`.
func docComment(indent, doc string) string {
	if doc == "" {
		return ""
	}
	text := strings.ReplaceAll(doc, commentEnd, commentEndEscaped)
	if !strings.Contains(text, newline) {
		return indent + docOpen + space + text + space + docClose + newline
	}
	var b strings.Builder
	b.WriteString(indent + docOpen + newline)
	for line := range strings.SplitSeq(text, newline) {
		if line == "" {
			b.WriteString(indent + docStarEmpty + newline)
			continue
		}
		b.WriteString(indent + docStar + line + newline)
	}
	b.WriteString(indent + docEnd + newline)
	return b.String()
}

// intText is an integer as a TypeScript number literal: decimal while it is a safe integer, else its nearest double in ECMAScript form (CONFORMANCE.md §4).
func intText(n int64) string {
	if n >= -maxSafeInt && n <= maxSafeInt {
		return strconv.FormatInt(n, decimal)
	}
	return floatText(float64(n))
}

// floatText is a double as ECMAScript's Number::toString gives it; negative zero is `-0` (CONFORMANCE.md §5).
func floatText(f float64) string {
	if f == 0 && math.Signbit(f) {
		return negativeZero
	}
	return types.FloatText(f, float64Bits)
}

// bigText is an integer as a bigint literal.
func bigText(n int64) string { return strconv.FormatInt(n, decimal) + bigSuffix }
