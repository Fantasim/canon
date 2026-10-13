package std

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// Format writes an Int or Float with a format spec: digits, `,`, sign, padding (STDLIB.md §9.5).
func Format(v value.Value, s syntax.FormatSpec) string { return Prepare(v, s).String() }

// Formatted is a formatted number in parts: its length is known before its text is built (§1.3).
type Formatted struct {
	sign, digits string
	short        int
	zeros        bool
}

// Prepare is v with s up to the padding, which String writes: the sign, the digits, `,` (§9.5).
func Prepare(v value.Value, s syntax.FormatSpec) Formatted {
	text := digitsOf(v, s)
	neg := strings.HasPrefix(text, minus)
	digits := strings.TrimPrefix(text, minus)
	if s.Comma && !strings.ContainsAny(digits, exponent) {
		digits = group(digits)
	}
	sign := ""
	switch {
	case neg && !allZero(digits):
		sign = minus
	case s.Plus:
		sign = plus
	}
	return Formatted{sign: sign, digits: digits, short: max(s.Width-len(sign)-len(digits), 0), zeros: s.Zero}
}

// Len is the length in bytes of the text String writes.
func (f Formatted) Len() int { return len(f.sign) + len(f.digits) + f.short }

// String pads to the width, zeros after the sign with `0`, else spaces before (STDLIB.md §9.5 step 4).
func (f Formatted) String() string {
	if f.zeros {
		return f.sign + strings.Repeat(zero, f.short) + f.digits
	}
	return strings.Repeat(space, f.short) + f.sign + f.digits
}

// digitsOf is an Int's sign and hex magnitude with `x`/`X`, else fixed's text (STDLIB.md §9.5 step 1).
func digitsOf(v value.Value, s syntax.FormatSpec) string {
	x, isInt := v.(*value.Int)
	if s.Hex == 0 || !isInt {
		return fixed(v, s.Decimals)
	}
	text := strconv.FormatInt(x.V, hexBase)
	if s.Hex == syntax.HexUpper {
		return strings.ToUpper(text)
	}
	return text
}

// fixed is the value with exactly n decimals, or its canonical form when n < 0.
func fixed(v value.Value, n int) string {
	switch x := v.(type) {
	case *value.Int:
		s := strconv.FormatInt(x.V, decimalBase)
		if n > 0 {
			s += point + strings.Repeat(zero, n)
		}
		return s
	case *value.Float:
		if n < 0 {
			return x.CanonText()
		}
		return roundAway(x.V, n)
	}
	return v.CanonText()
}

// roundAway rounds the exact binary value of f to n decimals, ties away from zero.
func roundAway(f float64, n int) string {
	r := new(big.Rat).SetFloat64(f)
	neg := r.Sign() < 0
	r.Abs(r)
	scale := new(big.Int).Exp(big.NewInt(decimalBase), big.NewInt(int64(n)), nil)
	r.Mul(r, new(big.Rat).SetInt(scale))
	r.Add(r, big.NewRat(1, halfDenominator))
	q := new(big.Int).Quo(r.Num(), r.Denom())
	s := q.String()
	if len(s) <= n {
		s = strings.Repeat(zero, n-len(s)+1) + s
	}
	if n > 0 {
		s = s[:len(s)-n] + point + s[len(s)-n:]
	}
	if neg {
		return minus + s
	}
	return s
}

// group inserts `,` between groups of three digits of the integer part.
func group(digits string) string {
	intPart, frac, hasFrac := strings.Cut(digits, point)
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%groupSize == 0 {
			b.WriteString(comma)
		}
		b.WriteRune(c)
	}
	if hasFrac {
		b.WriteString(point + frac)
	}
	return b.String()
}

// allZero reports a number whose digits are all zero, printed without `-` (STDLIB.md §9.5).
func allZero(digits string) bool {
	return strings.Trim(digits, zeroDigits) == ""
}
