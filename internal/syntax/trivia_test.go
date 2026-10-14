package syntax_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
)

var triviaNames = map[syntax.TriviaKind]string{
	syntax.TriviaSpace: "sp", syntax.TriviaNewline: "nl", syntax.TriviaLineComment: "line",
	syntax.TriviaBlockComment: "block", syntax.TriviaDocComment: "doc", syntax.TriviaBOM: "bom",
}

// GRAMMAR.md §2.2 GRM-08, DECISIONS 73: each token's leading and trailing trivia kinds on edge inputs.
func TestTriviaKinds(t *testing.T) {
	tests := []struct{ name, text, want string }{
		{"doc after code", "a = 1 /// x\n",
			"1 IDENT L[] T[sp]; 2 = L[] T[sp]; 3 INT L[] T[sp line]; 5 EOF L[nl] T[]"},
		{"doc after a block comment", "/* a\n b */ /// x\n", "1 EOF L[block sp line nl] T[]"},
		{"doc after a BOM", "\xEF\xBB\xBF/// x\na = 1\n",
			"1 IDENT L[bom doc nl] T[sp]; 2 = L[] T[sp]; 5 EOF L[nl] T[]"},
		{"doc after a carriage return", "\r/// x\na = 1\n",
			"1 IDENT L[sp doc nl] T[sp]; 2 = L[] T[sp]; 5 EOF L[nl] T[]"},
		{"four slashes", "//// x\na = 1\n", "1 IDENT L[line nl] T[sp]; 2 = L[] T[sp]; 5 EOF L[nl] T[]"},
		{"unterminated block", "a = 1 /* x\ny", "1 IDENT L[] T[sp]; 2 = L[] T[sp]; 3 INT L[] T[sp block]"},
		{"line break in an interpolation", "a = \"a{b\n/// x\n",
			"1 IDENT L[] T[sp]; 2 = L[] T[sp]; 6 EOF L[nl doc nl] T[]"},
		{"comment in a multiline interpolation", "a = \"\"\"\n{x /// c\n\"\"\"\n",
			"1 IDENT L[] T[sp]; 2 = L[] T[sp]; 4 IDENT L[] T[sp line]; 7 EOF L[nl] T[]"},
		{"no final line break", "a = 1 // c", "1 IDENT L[] T[sp]; 2 = L[] T[sp]; 3 INT L[] T[sp line]"},
		{"only comments", "// a\n/* b */ /// c\n\n/// d\n", "1 EOF L[line nl block sp line nl nl doc nl] T[]"},
	}
	for _, tt := range tests {
		f, _, _ := parseText(t, "a/a.canon", []byte(tt.text))
		if got := triviaLayout(f); got != tt.want {
			t.Errorf("%s: %q\n got %s\nwant %s", tt.name, tt.text, got, tt.want)
		}
	}
}

// triviaLayout is "index kind L[…] T[…]" for each token with trivia.
func triviaLayout(f *syntax.File) string {
	var out []string
	for i, tok := range f.Tokens {
		lead, trail := f.AppendLeading(nil, syntax.Tok(i)), f.AppendTrailing(nil, syntax.Tok(i))
		if len(lead)+len(trail) > 0 {
			out = append(out, fmt.Sprintf("%d %s L%s T%s", i, tok.Kind, triviaList(lead), triviaList(trail)))
		}
	}
	return strings.Join(out, "; ")
}

func triviaList(ts []syntax.Trivia) []string {
	out := make([]string, len(ts))
	for i, tr := range ts {
		out[i] = triviaNames[tr.Kind]
	}
	return out
}
