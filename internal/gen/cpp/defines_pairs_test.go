package cppgen

import (
	"maps"
	"math/rand"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const (
	pairsBoundary  = `\b`
	pairsSeed      = 1
	pairsTexts     = 20000
	pairsMaxPieces = 32
	pairsAllRuns   = -1
)

// pairsAlphabet are the pieces the property test glues into texts: names, separators and non-ASCII neighbours.
var pairsAlphabet = []string{"E", "M", "M1", "_", ":", "::", " ", "\n", "(", "é", "EE", "1", ".", "<", "::E"}

// pairsNames are the (enum, macro) pairs every text is asked about.
var pairsNames = [][2]string{{"E", "M"}, {"E", "M1"}, {"EE", "M"}, {"E", "_"}, {"M", "E"}, {"E", "E"}, {"_", "M"}, {"a", "b"}, {"b", "c"}, {"a", "c"}}

// pairsWord is a maximal run of the bytes \b treats as a word.
var pairsWord = regexp.MustCompile(`[0-9A-Za-z_]+`)

// pairsName is an asked pair's key and its \bEnum::MACRO\b, what the clash detection compiled per member before.
type pairsName struct {
	key string
	re  *regexp.Regexp
}

// pairsNamed are pairsNames compiled once; the keys a text brings are compiled per text, never cached, so a fuzzer cannot grow them.
var pairsNamed = func() []pairsName {
	out := make([]pairsName, 0, len(pairsNames))
	for _, n := range pairsNames {
		key := n[0] + scopeSep + n[1]
		out = append(out, pairsName{key, wordRegexp(key)})
	}
	return out
}()

// wordRegexp is \bkey\b.
func wordRegexp(key string) *regexp.Regexp {
	return regexp.MustCompile(pairsBoundary + regexp.QuoteMeta(key) + pairsBoundary)
}

// agrees fails the test unless the pair set and \bkey\b give one answer for the asked pairs, every two consecutive word runs of the text, and every key of the set. A match of \bA::B\b has A and B whole word runs with only :: between them, so the consecutive runs are every key the regular expression can match.
func agrees(t *testing.T, text string) {
	t.Helper()
	set := qualifiedPairs(text)
	for _, n := range pairsNamed {
		agreesOn(t, text, set, n)
	}
	runs := pairsWord.FindAllString(text, pairsAllRuns)
	keys := slices.Collect(maps.Keys(set))
	for i := 1; i < len(runs); i++ {
		keys = append(keys, runs[i-1]+scopeSep+runs[i])
	}
	slices.Sort(keys)
	for _, key := range slices.Compact(keys) {
		agreesOn(t, text, set, pairsName{key, wordRegexp(key)})
	}
}

// agreesOn fails the test unless n's regular expression matches text exactly when n's key is in set.
func agreesOn(t *testing.T, text string, set map[string]bool, n pairsName) {
	t.Helper()
	if want, got := n.re.MatchString(text), set[n.key]; want != got {
		t.Fatalf("%q, %s: regexp %v, pair set %v", text, n.key, want, got)
	}
}

// pairsCases are texts whose pairs the spelling rules of CODEGEN.md §7.9 decide: whole words only.
var pairsCases = []struct {
	name, text string
	want       []string
}{
	{"plain", "E::M", []string{"E::M"}},
	{"overlap", "a::b::c", []string{"a::b", "b::c"}},
	{"triple colon", "E:::M", nil},
	{"longer member", "E::M1", []string{"E::M1"}},
	{"longer enum", "EE::M", []string{"EE::M"}},
	{"global qualifier", "::ns::E::M", []string{"ns::E", "E::M"}},
	{"after text", "x = E::M;", []string{"E::M"}},
	{"non-ASCII before", "éE::M", []string{"E::M"}},
	{"non-ASCII after", "E::Mé", []string{"E::M"}},
	{"non-ASCII between", "E::éM", nil},
	{"space inside", "E :: M", nil},
	{"lone separator", "::", nil},
	{"empty", "", nil},
	{"digits and underscores", "_1::2_", []string{"_1::2_"}},
	{"many runs", "a::b c::d e::f g::h i::j k::l", []string{"a::b", "c::d", "e::f", "g::h", "i::j", "k::l"}},
}

// CODEGEN.md §7.9: qualifiedPairs holds exactly the `A::B` of whole words, text start and end included.
func TestQualifiedPairs(t *testing.T) {
	t.Parallel()
	for _, c := range pairsCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := qualifiedPairs(c.text)
			if len(got) != len(c.want) {
				t.Errorf("%q: %d pairs %v, want %v", c.text, len(got), got, c.want)
			}
			for _, w := range c.want {
				if !got[w] {
					t.Errorf("%q: %s missing from %v", c.text, w, got)
				}
			}
			agrees(t, c.text)
		})
	}
}

// CODEGEN.md §7.9: for seeded random texts the pair set and `\bA::B\b` give the same answer.
func TestQualifiedPairsMatchRegexp(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(pairsSeed))
	for range pairsTexts {
		var b strings.Builder
		for range r.Intn(pairsMaxPieces) {
			b.WriteString(pairsAlphabet[r.Intn(len(pairsAlphabet))])
		}
		agrees(t, b.String())
	}
}

// CODEGEN.md §7.9: the same equivalence over fuzzer-chosen texts.
func FuzzQualifiedPairs(f *testing.F) {
	for _, c := range pairsCases {
		f.Add(c.text)
	}
	f.Fuzz(func(t *testing.T, text string) { agrees(t, text) })
}
