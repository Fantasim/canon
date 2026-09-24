package std

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/value"
)

// Spec is a format spec `{x:spec}` (STDLIB.md §9.5); Decimals is syntax.NoDecimals without `.N`.
type Spec struct {
	Plus, Comma bool
	Decimals    int
}

// Format writes an Int or Float with a spec (STDLIB.md §9.5).
func Format(v value.Value, s Spec) string {
	text := fixed(v, s.Decimals)
	neg := strings.HasPrefix(text, minus)
	digits := strings.TrimPrefix(text, minus)
	if s.Comma && !strings.ContainsAny(digits, exponent) {
		digits = group(digits)
	}
	switch {
	case neg && !allZero(digits):
		return minus + digits
	case s.Plus:
		return plus + digits
	}
	return digits
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
