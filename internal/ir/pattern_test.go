package ir_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/ir"
)

// multiByte is every code point of two, three and four bytes, in the compact form exact on
// valid UTF-8, the tail `.`, negated classes, `\D`, `\W` and `\S` share.
const multiByte = `|[\xc2-\xdf][\x80-\xbf]|[\xe0-\xef][\x80-\xbf]{2}|[\xf0-\xf4][\x80-\xbf]{3}`

// TestCppPatternConstructs is EVALUATION.md §11.3's portable subset, one construct per row, under log 2026-09-24 "Pattern semantics": `.`, a negated class and `\D \W \S` are one code point (`.` but `\n`), `\s` is `[\t\n\f\r ]`, non-ASCII class members UTF-8 alternations; letters, digits and `_` stay raw, any other byte is `\xHH`.
func TestCppPatternConstructs(t *testing.T) {
	cases := []struct{ name, pattern, want string }{
		{"literal", `abc_1`, `abc_1`},
		{"escaped metacharacters", `\.\*\+\?\(\)\[\]\{\}\|\^\$\\`, `\x2e\x2a\x2b\x3f\x28\x29\x5b\x5d\x7b\x7d\x7c\x5e\x24\x5c`},
		{"multi-byte literal", `é`, `\xc3\xa9`},
		{"dot", `.`, `[\x00-\x09\x0b-\x7f]` + multiByte},
		{"class with ranges", `[a-c0-9_]`, `[0-9_a-c]`},
		{"negated class", `[^a]`, `[\x00-\x60b-\x7f]` + multiByte},
		{"non-ASCII class members", `[é-ü]`, `\xc3[\xa9-\xbc]`},
		{"mixed class", `[aé😀]`, `a|\xc3\xa9|\xf0\x9f\x98\x80`},
		{`\d`, `\d`, `[0-9]`},
		{`\D`, `\D`, `[\x00-\x2f\x3a-\x7f]` + multiByte},
		{`\w`, `\w`, `[0-9A-Z_a-z]`},
		{`\W`, `\W`, `[\x00-\x2f\x3a-\x40\x5b-\x5e\x60\x7b-\x7f]` + multiByte},
		{`\s`, `\s`, `[\x09-\x0a\x0c-\x0d\x20]`},
		{`\S`, `\S`, `[\x00-\x08\x0b\x0e-\x1f\x21-\x7f]` + multiByte},
		{"anchors", `^a$`, `^a$`},
		{"group", `(ab)c`, `abc`},
		{"non-capturing group", `(?:ab)+`, `(?:ab)+`},
		{"quote", `"`, `\x22`},
		{"alternation", `ab|cd`, `ab|cd`},
		{"alternation in a concatenation", `x(ab|cd)y`, `x(?:ab|cd)y`},
		{"star plus quest", `a*b+c?`, `a*b+c?`},
		{"counted", `a{2}b{2,}c{2,3}`, `a{2}b{2,}c{2,3}`},
		{"lazy, dropped", `a*?b+?c??d{2,3}?`, `a*b+c?d{2,3}`},
		{"quantified multi-byte", `é+`, `(?:\xc3\xa9)+`},
		{"quantified class alternation", `[aé]?`, `(?:a|\xc3\xa9)?`},
		{"quantified anchor", `^*a`, `(?:^)*a`},
		{"empty branch", `a|`, `a|(?:)`},
		{"any code point", `[\s\S]`, `[\x00-\x7f]` + multiByte},
		{"part of a length, exact", `[^ü]`, `[\x00-\x7f]|\xc2[\x80-\xbf]|\xc3[\x80-\xbb]|\xc3[\xbd-\xbf]|[\xc4-\xdf][\x80-\xbf]` +
			`|[\xe0-\xef][\x80-\xbf]{2}|[\xf0-\xf4][\x80-\xbf]{3}`},
		{"empty class (RE2 spelling)", `[^\x00-\x{10FFFF}]`, `\xff`},
		{"surrogate literal (RE2 spelling)", `a\x{D800}`, `\xff`},
		{"surrogates skipped (RE2 spelling)", `[\x{D7FF}-\x{E000}]`, `\xed\x9f\xbf|\xee\x80\x80`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ir.CppPattern(regexp.MustCompile(c.pattern))
			if err != nil || got != c.want {
				t.Errorf("CppPattern(%q) = %q, %v; want %q", c.pattern, got, err, c.want)
			}
		})
	}
}

// translatorRefused are the patterns CppPattern has no translation for.
var translatorRefused = []string{`(?i)a`, `(?m)^a`, `(?m)a$`, `\ba`, `a\B`}

// TestCppPatternRefuses is what has no translation: case folding, multi-line anchors and word boundaries, all outside EVALUATION.md §11.3's subset (E1904 refuses `(?i`, `(?m`, `\b`, `\B`).
func TestCppPatternRefuses(t *testing.T) {
	for _, p := range translatorRefused {
		if got, err := ir.CppPattern(regexp.MustCompile(p)); !errors.Is(err, ir.ErrPattern) {
			t.Errorf("CppPattern(%q) = %q, %v; want ErrPattern", p, got, err)
		}
	}
}

// TestCppPatternBytes is log 2026-09-24 "Pattern semantics" judged in Go: over every pair of
// the corpus, RE2 running the translation on the text's bytes (widened to Latin-1) accepts
// exactly when RE2 runs the pattern on the text; the translation is printable ASCII without `"` or `??`.
func TestCppPatternBytes(t *testing.T) {
	patterns, inputs := fullCorpus()
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		tr, err := ir.CppPattern(re)
		if err != nil {
			t.Fatalf("CppPattern(%q): %v", p, err)
		}
		assertBytesAgree(t, re, tr, inputs)
	}
}

// assertBytesAgree fails when tr is not printable ASCII, or RE2 disagrees on one input.
func assertBytesAgree(t *testing.T, re *regexp.Regexp, tr string, inputs []string) {
	t.Helper()
	if strings.IndexFunc(tr, func(r rune) bool { return r < ' ' || r >= utf8.RuneSelf-1 || r == '"' }) >= 0 || strings.Contains(tr, "??") {
		t.Fatalf("CppPattern(%q) = %q: not printable ASCII, or holds `\"` or `??`", re, tr)
	}
	bytes := regexp.MustCompile(tr)
	for _, in := range inputs {
		if want, got := re.MatchString(in), bytes.MatchString(latin1(in)); want != got {
			t.Errorf("pattern %q, translation %q, input %q: RE2 %v, translation %v", re, tr, in, want, got)
		}
	}
}
