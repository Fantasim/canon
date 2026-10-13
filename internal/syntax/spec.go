package syntax

import (
	"strconv"
	"strings"
)

// parseSpec reads a format spec, `\+?(0?[1-9][0-9]?)?,?(\.[0-9]{1,2})?[xX]?` (GRAMMAR.md §2.6).
func parseSpec(spec []byte) (FormatSpec, bool) {
	fs := FormatSpec{Decimals: NoDecimals}
	s := string(spec)
	s, fs.Plus = strings.CutPrefix(s, specPlus)
	s, fs.Zero = strings.CutPrefix(s, specZero)
	s, fs.Width = specWidth(s)
	s, fs.Comma = strings.CutPrefix(s, specComma)
	s, ok := specDecimals(s, &fs)
	s, fs.Hex = specHex(s)
	return fs, ok && s == "" && len(spec) > 0 && specCombines(fs)
}

// specWidth reads a width, 1 to 99 without a leading zero; 0 when there is none.
func specWidth(s string) (string, int) {
	digits, rest := digitPrefix(s)
	if digits == "" || digits[0] == specZero[0] {
		return s, 0
	}
	w, err := strconv.Atoi(digits)
	if err != nil {
		return s, 0
	}
	return rest, w
}

// specDecimals reads ".N", N of 1 or 2 digits and at most 20; false: a "." without valid digits.
func specDecimals(s string, fs *FormatSpec) (string, bool) {
	after, dot := strings.CutPrefix(s, fractionDot)
	if !dot {
		return s, true
	}
	digits, rest := digitPrefix(after)
	n, err := strconv.Atoi(digits)
	if err != nil || n > maxDecimals {
		return s, false
	}
	fs.Decimals = n
	return rest, true
}

// specHex reads a final "x" or "X"; 0 when there is none.
func specHex(s string) (string, byte) {
	if len(s) == 1 && (s[0] == hexLower || s[0] == HexUpper) {
		return "", s[0]
	}
	return s, 0
}

// specCombines refuses the parts that do not go together: `0` with no width or with `,`, and
// `x`/`X` with `,` or `.N` (DECISIONS 358).
func specCombines(fs FormatSpec) bool {
	if fs.Zero && (fs.Width == 0 || fs.Comma) {
		return false
	}
	return fs.Hex == 0 || (!fs.Comma && fs.Decimals == NoDecimals)
}

// digitPrefix splits the leading decimal digits of s, at most maxSpecDigits of them, from the rest.
func digitPrefix(s string) (string, string) {
	n := 0
	for n < len(s) && n < maxSpecDigits && isDigit(s[n]) {
		n++
	}
	return s[:n], s[n:]
}
