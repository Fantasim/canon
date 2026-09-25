package progen_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// fixtureTarget is src as the one file of package m.
func fixtureTarget(src string) target { return metaTarget(metaPath, src) }

// editTexts are the texts a site's edits write, in order.
func editTexts(s progen.Site) []string {
	out := make([]string, 0, len(s.Edits))
	for _, e := range s.Edits {
		out = append(out, e.Text)
	}
	return out
}

// The mutators the review and the nightly found writing a finding of their own, each with a
// fixture of the construct they must leave alone and of one they still mutate.
func TestMutatorsAvoidTheirOwnFindings(t *testing.T) {
	for _, tc := range []struct {
		rule, src string
		sites     func(target) []progen.Site
		want      []string // each site's focus text, in order
	}{
		{
			"TYPES.md §5.1: no misspelling beside a context-dependent operand",
			"package m\nenum Tier ordered { low, high }\nfn f(tier: Tier, n: Int) -> Bool { return tier >= high and n > 0 and n == n }\n",
			unknownName, []string{"zzn", "zzn"},
		},
		{
			"TYPES.md §7.2: a code past its own @codes type, none past Int",
			"package m\nenum E @codes(UInt16) { a = 1 }\nenum F @codes(Int) { b = 1 }\n",
			codeOutOfRange, []string{"65536"},
		},
		{
			"decision log Check C2: none in a @codes enum gets a free code",
			"package m\nenum E @codes(UInt8) { a = 1, b = 2 }\nenum P { c }\n",
			noneMember, []string{"none", "none"},
		},
		{
			"DECISIONS 214: no self-containing record with input fields",
			"package m\nrecord A { n: Int }\nrecord B { port: input Int }\n",
			selfRecord(""), []string{"A"},
		},
		{
			"TYPES.md narrowing: no chain onto a test against none",
			"package m\nfn f(x: Int?, y: Int) -> Bool { return x != none and y < 2 }\n",
			chainedComparison, []string{"=="},
		},
		{
			"TYPES.md narrowing: no brace literal in a header that narrows",
			"package m\nfn f(x: Int?, y: Int) -> Int {\n  if x != none { return 1 }\n  if y < 2 { return 2 }\n  return 3\n}\n",
			braceInHeader, []string{"{}"},
		},
		{
			"TYPES.md §7.4: a string its pattern refuses",
			"package m\nrecord R { id: String(/^[a-z]+$/) }\nlet r: R = { id: \"abc\" }\n",
			patternMismatch, []string{`""`},
		},
	} {
		sites := tc.sites(fixtureTarget(tc.src))
		var got []string
		for _, s := range sites {
			got = append(got, editTexts(s)[s.Focus])
		}
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s: focus texts %q, want %q", tc.rule, got, tc.want)
		}
	}
}

// decision log Check C2: in a @codes enum, "none" comes with the least free power of two.
func TestNoneMemberCode(t *testing.T) {
	sites := noneMember(fixtureTarget("package m\nenum E @codes(UInt8) { a = 1, b = 2 }\n"))
	if len(sites) != 1 || editTexts(sites[0])[2] != " = 4" {
		t.Fatalf("sites %+v, want none = 4", sites)
	}
}

// STDLIB.md: load.defines is a table, whose members are keys, never an E3003 site.
func TestCollectionsHoldDefines(t *testing.T) {
	tg := fixtureTarget("package m\nlocal let objH = load.defines(\"@r/a.h\")\n")
	if !collections(tg)["objH"] {
		t.Fatal("a load.defines let is a collection")
	}
}

// CODEGEN.md §2.2 (E8015), WIRE.md §8.1 (E8153): data or embedded mode, whatever the emit's target.
func TestDataModeCoversEveryCodeEmit(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want bool
	}{
		{"package m\nemit cpp { out: \"o/\", namespace: \"m\", mode: data }\n", true},
		{"package m\nemit go { out: \"o/\", package: \"m\", mode: embedded }\n", true},
		{"package m\nemit go { out: \"o/\", package: \"m\", mode: baked }\n", false},
	} {
		if got := dataMode(fixtureTarget(tc.src)); got != tc.want {
			t.Errorf("dataMode(%q) = %v, want %v", tc.src, got, tc.want)
		}
	}
}

// TYPES.md §5.2 (E3301, E3321): a map literal's identifier keys are no fields to add or repeat.
func TestRecordLiteralsSkipMaps(t *testing.T) {
	src := "package m\nenum K { x, y }\nrecord R { /// n.\n  n: Int }\nlet r: R = { n: 1 }\nlet k: {K: Int} = { x: 1, y: 2 }\n"
	got := recordLiterals(fixtureTarget(src))
	if len(got) != 1 {
		t.Fatalf("%d record literals, want 1 (the map literal is none)", len(got))
	}
}
