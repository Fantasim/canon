package cppgen

import (
	"math/rand"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const (
	tokensBoundary  = `\b`
	tokensSeed      = 1
	tokensTexts     = 20000
	tokensMaxPieces = 32
)

// tokensAlphabet are the pieces the property test glues into texts: names, separators and non-ASCII neighbours.
var tokensAlphabet = []string{"E", "M", "M1", "_", ":", "::", " ", "\n", "(", "é", "EE", "1", ".", "<", "\"", "/*"}

// tokensNames are the macro names every text is asked about; "_" and "M1" test word edges.
var tokensNames = []string{"E", "M", "M1", "EE", "_", "a", "b", "E1", "_M"}

// tokensSet is tokensNames as the set the generator looks tokens up in.
var tokensSet = func() map[string]bool {
	out := map[string]bool{}
	for _, n := range tokensNames {
		out[n] = true
	}
	return out
}()

// tokensRegexps are \bname\b of each name, what the wrap would be if it ran one regular expression per macro (the quadratic form DECISIONS 391 replaced).
var tokensRegexps = func() map[string]*regexp.Regexp {
	out := map[string]*regexp.Regexp{}
	for _, n := range tokensNames {
		out[n] = regexp.MustCompile(tokensBoundary + regexp.QuoteMeta(n) + tokensBoundary)
	}
	return out
}()

// agrees fails the test unless macrosIn finds exactly the names whose \bname\b matches text, sorted.
func agrees(t *testing.T, text string) {
	t.Helper()
	var want []string
	for _, n := range tokensNames {
		if tokensRegexps[n].MatchString(text) {
			want = append(want, n)
		}
	}
	slices.Sort(want)
	if got := macrosIn(tokensSet, text); !slices.Equal(got, want) {
		t.Fatalf("%q: macrosIn %v, regexps %v", text, got, want)
	}
}

// tokensCases are texts whose tokens the spelling rules of CODEGEN.md §7.9 decide: whole identifiers only.
var tokensCases = []struct {
	name, text string
	want       []string
}{
	{"plain", "E", []string{"E"}},
	{"qualified", "E::M", []string{"E", "M"}},
	{"longer name", "EE M1", []string{"EE", "M1"}},
	{"inside a longer word", "xE Ex E_ _E", nil},
	{"underscore name", "a + _", []string{"_", "a"}},
	{"global qualifier", "::ns::E::M", []string{"E", "M"}},
	{"non-ASCII neighbour", "éE Eé", []string{"E"}},
	{"digit neighbour", "1E E1 2M", []string{"E1"}},
	{"in a string", `"M"`, []string{"M"}},
	{"empty", "", nil},
	{"one name twice", "M M M", []string{"M"}},
}

// CODEGEN.md §7.9, DECISIONS 391: macrosIn holds exactly the whole identifier tokens of the text that are in the set, sorted and distinct.
func TestMacrosIn(t *testing.T) {
	t.Parallel()
	for _, c := range tokensCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := macrosIn(tokensSet, c.text)
			if !slices.Equal(got, c.want) {
				t.Errorf("%q: %v, want %v", c.text, got, c.want)
			}
			agrees(t, c.text)
		})
	}
}

// CODEGEN.md §7.9: for seeded random texts the token scan and `\bname\b` give the same answer.
func TestMacrosInMatchRegexp(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(tokensSeed))
	for range tokensTexts {
		var b strings.Builder
		for range r.Intn(tokensMaxPieces) {
			b.WriteString(tokensAlphabet[r.Intn(len(tokensAlphabet))])
		}
		agrees(t, b.String())
	}
}

// CODEGEN.md §7.9: the same equivalence over fuzzer-chosen texts.
func FuzzMacrosIn(f *testing.F) {
	for _, c := range tokensCases {
		f.Add(c.text)
	}
	f.Fuzz(func(t *testing.T, text string) { agrees(t, text) })
}

// CODEGEN.md §7.9: a text names no macro of an empty set.
func TestMacrosInEmptySet(t *testing.T) {
	t.Parallel()
	if got := macrosIn(nil, "E M"); got != nil {
		t.Errorf("macrosIn(nil) = %v", got)
	}
}
