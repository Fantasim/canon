package syntax

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// grammarSpec is GRAMMAR.md §2.6's format spec pattern, verbatim.
var grammarSpec = regexp.MustCompile(`^\+?(0?[1-9][0-9]?)?,?(\.[0-9]{1,2})?[xX]?$`)

// specByGrammar is GRAMMAR.md §2.6 read literally: the pattern, non-empty, and its three rules.
func specByGrammar(s string) bool {
	if s == "" || !grammarSpec.MatchString(s) {
		return false
	}
	if _, n, ok := strings.Cut(s, "."); ok {
		if d, err := strconv.Atoi(strings.TrimRight(n, "xX")); err != nil || d > 20 {
			return false
		}
	}
	hex := strings.ContainsAny(s, "xX")
	zero := strings.HasPrefix(strings.TrimPrefix(s, "+"), "0")
	return !(hex && strings.ContainsAny(s, ",.")) && !(zero && strings.Contains(s, ","))
}

// GRAMMAR.md §2.6, DECISIONS 358: parseSpec agrees with the grammar on every short spec string.
func TestParseSpecMatchesGrammar(t *testing.T) {
	const alphabet = "+0129,.xX"
	var walk func(prefix string)
	walk = func(prefix string) {
		if _, got := parseSpec([]byte(prefix)); got != specByGrammar(prefix) {
			t.Errorf("parseSpec(%q) ok = %t, GRAMMAR.md §2.6 says %t", prefix, got, !got)
		}
		if len(prefix) == 5 {
			return
		}
		for i := range len(alphabet) {
			walk(prefix + alphabet[i:i+1])
		}
	}
	walk("")
}

// GRAMMAR.md §2.6, DECISIONS 358: the parts of a spec.
func TestParseSpecParts(t *testing.T) {
	for _, c := range []struct {
		spec string
		want FormatSpec
	}{
		{"08x", FormatSpec{Decimals: NoDecimals, Zero: true, Width: 8, Hex: 'x'}},
		{"02X", FormatSpec{Decimals: NoDecimals, Zero: true, Width: 2, Hex: 'X'}},
		{"x", FormatSpec{Decimals: NoDecimals, Hex: 'x'}},
		{"+03", FormatSpec{Decimals: NoDecimals, Plus: true, Zero: true, Width: 3}},
		{"12,", FormatSpec{Decimals: NoDecimals, Width: 12, Comma: true}},
		{"5", FormatSpec{Decimals: NoDecimals, Width: 5}},
		{"99", FormatSpec{Decimals: NoDecimals, Width: 99}},
		{"010.2", FormatSpec{Zero: true, Width: 10, Decimals: 2}},
		{"+,.20", FormatSpec{Plus: true, Comma: true, Decimals: 20}},
		{".0", FormatSpec{Decimals: 0}},
	} {
		got, ok := parseSpec([]byte(c.spec))
		if !ok || got != c.want {
			t.Errorf("parseSpec(%q) = %+v, %t; want %+v", c.spec, got, ok, c.want)
		}
	}
	for _, bad := range []string{"x,", ",x", ".2x", "0,5", "05,", "0", "+0", "00", "100", "0x5", "8xx", "-5", " 5"} {
		if _, ok := parseSpec([]byte(bad)); ok {
			t.Errorf("parseSpec(%q) accepted; GRAMMAR.md §2.6 refuses it", bad)
		}
	}
}
