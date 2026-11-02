package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// DECISIONS 215, WIRE.md §6.1: a load path holding a lexer error is not judged again.
func TestLoadPathLexError(t *testing.T) {
	const head = "package a\n\nconst DIR = \"data\"\n\n/// Limits.\nrecord Limits {\n  /// A cap.\n  n: Int = 0\n}\n\n"
	cases := []struct {
		name, let    string
		e1112, e7008 int
	}{
		{"load", `local let limits: Limits = load("{}data/limits.json")`, 1, 0},
		{"load.text", `local let note: String = load.text("{}data/note.txt")`, 1, 0},
		{"load.dir", `local let all: [Limits] = load.dir("{}data/*.json")`, 1, 0},
		{"sound interpolation", `local let named: String = load.text("{DIR}/note.txt")`, 0, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, bag := checkKeepingBags(t, head+tc.let+"\n")
			counts := map[diag.Code]int{}
			for _, f := range bag.Findings() {
				counts[f.Code]++
			}
			if got := counts[diag.E1112.Def().Code]; got != tc.e1112 {
				t.Errorf("empty interpolations: got %d, want %d (%v)", got, tc.e1112, bag.Findings())
			}
			if got := counts[diag.E7008.Def().Code]; got != tc.e7008 {
				t.Errorf("paths that are no literal: got %d, want %d (%v)", got, tc.e7008, bag.Findings())
			}
		})
	}
}
