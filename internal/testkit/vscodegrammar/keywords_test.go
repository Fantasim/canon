package vscodegrammar

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

// reservedWords reads the fenced block under GRAMMAR.md 4.1.
func reservedWords(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(specGrammarPath)
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(string(data), reservedHeading)
	if !ok {
		t.Fatal("reserved words block not found in GRAMMAR.md")
	}
	_, block, ok := strings.Cut(after, fenceMarker)
	if !ok {
		t.Fatal("reserved words block not found in GRAMMAR.md")
	}
	block, _, _ = strings.Cut(block, fenceMarker)
	return strings.Fields(block)
}

func hasKeywordScope(scopes string) bool {
	for _, p := range keywordScopePrefixes {
		if strings.HasPrefix(scopes, p) {
			return true
		}
	}
	return false
}

// GRAMMAR.md 4.1: every reserved word, alone on a line, is one keyword-scoped token.
func TestEveryReservedWordHighlights(t *testing.T) {
	g, err := Load(grammarPath)
	if err != nil {
		t.Fatal(err)
	}
	words := reservedWords(t)
	if len(words) == 0 {
		t.Fatal("reserved words block not found in GRAMMAR.md")
	}
	for _, w := range words {
		got, err := g.Tokenize(w)
		if err != nil {
			t.Fatal(err)
		}
		_, scopes, _ := strings.Cut(strings.TrimSuffix(got, lineSeparator), " "+`"`+w+`" `)
		if !hasKeywordScope(scopes) {
			t.Errorf("reserved word %q is not highlighted as a keyword: %q", w, got)
		}
	}
}

// GRAMMAR.md 2.3: a reserved word inside an identifier is not a keyword.
func TestKeywordInsideIdentifierIsPlain(t *testing.T) {
	g, err := Load(grammarPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"main", "inside", "none_", "format", "_in", "record2"} {
		got, err := g.Tokenize(id)
		if err != nil {
			t.Fatal(err)
		}
		if got != "" {
			t.Errorf("identifier %q is scoped: %q", id, got)
		}
	}
}

// GRAMMAR.md 4.1: the grammar's keyword rules list exactly the reserved words, no more.
func TestKeywordSetIsExactlyTheReservedWords(t *testing.T) {
	data, err := os.ReadFile(grammarPath)
	if err != nil {
		t.Fatal(err)
	}
	var gj grammarJSON
	if err := json.Unmarshal(data, &gj); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range gj.Repository[keywordsRule].Patterns {
		list := strings.TrimSuffix(strings.TrimPrefix(p.Match, keywordPrefix), keywordSuffix)
		got = append(got, strings.Split(list, "|")...)
	}
	want := reservedWords(t)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("keyword rules %v differ from GRAMMAR.md 4.1 %v", got, want)
	}
}
