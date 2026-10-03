package progen_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/progen"
	"github.com/fantasim/canonlang/internal/testkit/progen/grammar"
)

// DECISIONS 200 (second wave): the grammar suites cycle through every kind of file.
func TestCaseKinds(t *testing.T) {
	seen := map[grammar.Kind]bool{}
	for k := range 2 * len(grammar.Kinds()) {
		seen[caseKind(k)] = true
	}
	if len(seen) != len(grammar.Kinds()) {
		t.Errorf("cases cover %d of %d kinds", len(seen), len(grammar.Kinds()))
	}
}

// DECISIONS 200: a kept case of each kind replays as its kind and passes its property.
func TestReplayKinds(t *testing.T) {
	for i, kind := range grammar.Kinds() {
		for _, suite := range []string{suiteGrammar, suiteCorrupt} {
			cc := programCase(suite, i, caseSeed(suite, i))
			if got := archiveKind(cc.c.Files); got != kind {
				t.Fatalf("%s %s: the archive reads as %s", suite, kind, got)
			}
		}
		src := generate(caseSeed(suiteGrammar, i), kind)
		files := progen.NewProject()
		files.Set(kind.File(), src)
		c := &progen.Counterexample{Suite: suiteGrammar, Name: propRoundTrip, Files: files}
		if v := replayProgram(c); v.Kind != "" {
			t.Errorf("%s: %s", kind, v.Text)
		}
	}
}

// GRAMMAR.md §1: a file only a byte order mark spoils passes the corruption property.
func TestCorruptionBOM(t *testing.T) {
	if v := corruption([]byte(bom+"package a\n"), grammar.Source); v.Kind != "" {
		t.Errorf("a file with a byte order mark: %s", v.Text)
	}
	if v := corruption([]byte(bom+"package a\nlet = 1\n"), grammar.Source); v.Kind != "" {
		t.Errorf("a file with a byte order mark and a syntax error: %s", v.Text)
	}
}
