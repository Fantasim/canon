package ir_test

import (
	"math/rand/v2"
	"strings"
)

// specPatterns are EVALUATION.md §11.3's subset alone and combined, over ASCII and multi-byte text: check accepts each.
var specPatterns = []string{
	`abc`, `^abc$`, `a.c`, `^.$`, `^..$`, `^...$`, `^.{2}$`, `.`, `^.*$`, `^.+$`, `^[a-z]+$`,
	`^[^a-z]+$`, `[^a]`, `^[^a]$`, `^[^a]+$`, `\d+`, `^\d$`, `^\D$`, `^\D+$`, `^\w+$`, `^\W$`,
	`^\W+$`, `\s`, `^\s$`, `^\S$`, `^\S+$`, `^\s+$`, `(ab|cd)+`, `^(?:ab|cd){2,3}$`, `a{2}`,
	`a{2,}`, `^a{2,3}$`, `a*?b`, `a+?`, `a??b`, `a{1,2}?c`, `^(a|)$`, `x*`, `^$`, `\.`, `^\.$`,
	`\*\+\?\(\)\[\]\{\}\|\^\$\\`, `a/b`, `^é$`, `é+`, `^é{2}$`, `^[é]$`, `^[à-ÿ]+$`, `^[^é]$`,
	`[α-ω]`, `^[€]$`, `^[😀-🙏]$`, `日本`, `^[a-zé€😀]{2}$`, `^.é.$`, `^[^日]$`, `^[^😀]+$`,
	`^[^a-zé]+$`, `a|^b`, `a$|b`, `(^a)*b`, `(?:^)*a`, `(?:$)+`, `(a*)*`, `(a|b*)*c`,
	`(a?){3}`, `((a)|b)+`, `(?:a{0})b`, `x{0}`, `^.{20}$`, `^\W{8}$`, `^[-a]$`, `^[a-]$`,
	`^[\^]$`, `^\d{3}-\d{4}$`, `^[A-Za-z_][A-Za-z0-9_]*$`, `^\S+@\S+\.\S+$`, `^.*?$`, `^(.)+$`,
	`[^\s\S]`, `^[\s\S]$`, `^\w\W$`, `^(é|e)+$`, `ab|ac|ad`, `a(?:b|c)d`,
}

// robustPatterns are RE2 spellings outside §11.3's subset that the automaton still handles exactly; only the corpus and fuzz tests use them.
var robustPatterns = []string{
	`^*a`, `^[^\x00-\x{10FFFF}]`, `[\x00-\x{10FFFF}]`, `^\x{10FFFF}$`, `^[\x{7F}-\x{80}]$`,
	`^[\x{7FF}-\x{800}]$`, `^[\x{FFFF}-\x{10000}]$`, `^[\x{D7FF}-\x{E000}]$`, `a\x{D800}`, `\x00`,
	`\t|\n|\v|\f|\r`, `\r`, `^[^\n]$`, `^[\n]$`, `\Q.*\E`, `\101`, `\x41`, `\x{263A}`, `(?-s:.)`,
	`[x[:digit:]]`, `^[]a]$`, `(?s:.)`, `(?s)^.$`, `^[^\x{800}-\x{FFFF}]$`, `\n\r+`,
}

// fixedInputs are texts each construct can tell apart: empty, ASCII, every UTF-8 length and
// boundary, `\n`, `\r`, `\v`, NBSP and the other spaces ECMAScript's `\s` has and RE2's has not.
var fixedInputs = []string{
	"", "a", "b", "c", "abc", "xabcx", "ab", "abcd", "ac", "ad", "abab", "cdcd", "ababab", "aa",
	"aaa", "aaaa", "aab", "é", "ée", "é", "àÿ", "àbc", "日本", "日本語", "日", "😀", "🙏",
	"😀🙏", "€", "é€", "α", "ω", "\n", "\r", "\t", "\v", "\f", " ", "\r\n", "a\nb", "a\rb",
	"\x00", "a\x00b", "\x7f", "\u0080", "߿", "ࠀ", "퟿", "", "￿",
	"\U00010000", "\U0010ffff", "�", "a/b", "A", "_", "-", "]", "^", "123", "555-1234",
	"1a", "é1", " ", " ", "　", "ÿ", "..", "*+?()[]{}|^$\\", ".*", "x.y", "aé",
	"éa", "éé", ".é.", "aéb", "a😀b", "☺", "a@b.c", "é@日.x", strings.Repeat("x", 20),
	strings.Repeat("é", 20), strings.Repeat("!", 8), strings.Repeat(" ", 8), "x\n",
}

// The shape of the generated corpus: a fixed seed, so every run judges the same pairs.
const (
	genSeed1, genSeed2 = 20260924, 113
	genPatterns        = 300
	genInputs          = 40
	genMaxDepth        = 3
	genMaxInput        = 7
	genShapes          = 6
	genAlts            = 3
)

// genAtoms and genAnchors are the leaves of generated patterns (an anchor is never quantified bare, which ECMAScript refuses); genRunes the letters of generated inputs.
var (
	genAtoms = []string{`a`, `b`, `é`, `日`, `😀`, `.`, `\.`, `[a-c]`, `[^a]`, `[^é]`, `[^日]`,
		`[é-ü]`, `\d`, `\D`, `\w`, `\W`, `\s`, `\S`, `[^a-c]`, `[a-z😀]`, ` `}
	genAnchors = []string{`^`, `$`}
	genQuants  = []string{`*`, `+`, `?`, `*?`, `+?`, `??`, `{2}`, `{0,2}`, `{1,}`, `{1,2}?`}
	genRunes   = []rune{'a', 'b', 'c', 'é', '日', '😀', '\n', '\r', '\v', ' ', '1', '_', '.', 0x80, 0xFFFF, 0xA0}
)

// generatedCorpus is genPatterns patterns and genInputs inputs, the same on every run.
func generatedCorpus() (patterns, inputs []string) {
	r := rand.New(rand.NewPCG(genSeed1, genSeed2))
	for range genPatterns {
		patterns = append(patterns, genPattern(r, genMaxDepth))
	}
	for range genInputs {
		var b strings.Builder
		for range r.IntN(genMaxInput + 1) {
			b.WriteRune(genRunes[r.IntN(len(genRunes))])
		}
		inputs = append(inputs, b.String())
	}
	return patterns, inputs
}

// fullCorpus is every pattern (spec, robustness, generated) and every input.
func fullCorpus() (patterns, inputs []string) {
	genPats, genIns := generatedCorpus()
	patterns = append(append(append([]string(nil), specPatterns...), robustPatterns...), genPats...)
	return patterns, append(append([]string(nil), fixedInputs...), genIns...)
}

// genPattern is a random pattern of the portable subset, nested at most depth deep.
func genPattern(r *rand.Rand, depth int) string {
	if depth == 0 {
		leaves := append(append([]string(nil), genAtoms...), genAnchors...)
		return leaves[r.IntN(len(leaves))]
	}
	switch r.IntN(genShapes) {
	case 0:
		return genPattern(r, depth-1) + genPattern(r, depth-1)
	case 1:
		parts := make([]string, 1+r.IntN(genAlts))
		for i := range parts {
			parts[i] = genPattern(r, depth-1)
		}
		return "(" + strings.Join(parts, "|") + ")"
	case 2:
		return "(?:" + genPattern(r, depth-1) + ")" + genQuants[r.IntN(len(genQuants))]
	case 3:
		return "^" + genPattern(r, depth-1)
	case 4:
		return genPattern(r, depth-1) + "$"
	}
	return genAtoms[r.IntN(len(genAtoms))] + genQuants[r.IntN(len(genQuants))]
}
