package syntax_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
)

// kinds is the token kinds of a parse of text, BOF and EOF included.
func kinds(t *testing.T, text string) []syntax.TokenKind {
	t.Helper()
	f, _, _ := parseText(t, "a.canon", []byte(text))
	out := make([]syntax.TokenKind, len(f.Tokens))
	for i, tok := range f.Tokens {
		out[i] = tok.Kind
	}
	return out
}

func kindNames(ks []syntax.TokenKind) string {
	names := make([]string, len(ks))
	for i, k := range ks {
		names[i] = k.String()
	}
	return strings.Join(names, " ")
}

// GRAMMAR.md §2, §3 and DECISIONS 73: the token stream of small inputs, NL separators included.
func TestTokenStream(t *testing.T) {
	tests := []struct{ rule, text, want string }{
		{
			"§2.6 interpolation pieces", `x = "a{b:,}c{ {d: 1}.d }e"` + "\n",
			`BOF IDENT = STRING_HEAD IDENT FORMAT_SPEC STRING_MID { IDENT : INT } . IDENT STRING_TAIL NL EOF`,
		},
		{"§2.6 multiline pieces", "x = \"\"\"\n  a {b}\n  \"\"\"\n", `BOF IDENT = MLSTRING_HEAD IDENT MLSTRING_TAIL NL EOF`},
		{"§2.6 raw strings", "x = r\"a\\{b}\" + r\"\"\"\n  c\n  \"\"\"", `BOF IDENT = RAW + RAWML EOF`},
		{"§2.7 regex after ( and ,", "f(/a/, /b/) / c", `BOF IDENT ( REGEX , REGEX ) / IDENT EOF`},
		{"§2.4 numbers", "1..2 1.5.abs() 0x1F 1e-3 90s", `BOF INT .. INT FLOAT . IDENT ( ) INT FLOAT DURATION EOF`},
		{"§2.8 longest match", "a!=b a! ?. ?? ..= ... #", `BOF IDENT != IDENT IDENT ! ?. ?? ..= ... ILLEGAL EOF`},
		{"§5.7 # after [", "a[#0]", `BOF IDENT [ # INT ] EOF`},
		{"§4.1 words", "check package _ _x", `BOF check package _ IDENT EOF`},
		{"§3.3 else continues", "check a\n  else \"m\"\nlet b = 1", `BOF check IDENT else STRING NL let IDENT = INT EOF`},
		{"§3.3 . continues", "let a = b\n  .c()\nlet d = 1", `BOF let IDENT = IDENT . IDENT ( ) NL let IDENT = INT EOF`},
		{"§3.1 rule 1: [ inside {, { inside [", "x = [\n  {\n    a: 1\n    b: 2\n  }\n  for c in d\n]", `BOF IDENT = [ { IDENT : INT NL IDENT : INT NL } for IDENT in IDENT ] EOF`},
		{"§3.1 rule 2 DECISIONS 302: keyword after . ends an item", "x = {\n  a: L.in\n  b: 1\n}", `BOF IDENT = { IDENT : IDENT . in NL IDENT : INT NL } EOF`},
		{"§3.1 rule 2 DECISIONS 302: x.in unchanged on one line", "x = {\n  a: y.in z\n}", `BOF IDENT = { IDENT : IDENT . in IDENT NL } EOF`},
		{"§3.1 rule 4: own-line annotation continues", "a: Int = none\n  @deprecated(\"x\")\nb: Int", `BOF IDENT : IDENT = none @ IDENT ( STRING ) NL IDENT : IDENT EOF`},
		{"§3.1 rule 4: prefix annotation", "x = 1\n@reload\n\nlet y = 2", `BOF IDENT = INT NL @ IDENT NL let IDENT = INT EOF`},
		{"§3.1 rule 2: an operator ends no line", "a = b +\n  c\nd = 1", `BOF IDENT = IDENT + IDENT NL IDENT = INT EOF`},
		{"§3.1 notes: a comment line and a blank line make one NL", "a = 1\n\n// c\n\nb = 2", `BOF IDENT = INT NL IDENT = INT EOF`},
		{"§2.2 doc comments are trivia", "/// d\npackage a", `BOF package IDENT EOF`},
	}
	for _, tt := range tests {
		if got := kindNames(kinds(t, tt.text)); got != tt.want {
			t.Errorf("%s: %q\n got %s\nwant %s", tt.rule, tt.text, got, tt.want)
		}
	}
}

// DECISIONS 73: Trailing runs to the first line break, Leading holds the rest; NL is an empty
// token at their border.
func TestTriviaSplit(t *testing.T) {
	f, _, _ := parseText(t, "a.canon", []byte("a = 1 /* c */ // d\n\n// e\nb = 2\n"))
	one := f.Tokens[3]
	nl, b := f.Tokens[4], f.Tokens[5]
	trailing := []syntax.TriviaKind{syntax.TriviaSpace, syntax.TriviaBlockComment, syntax.TriviaSpace, syntax.TriviaLineComment}
	leading := []syntax.TriviaKind{syntax.TriviaNewline, syntax.TriviaNewline, syntax.TriviaLineComment, syntax.TriviaNewline}
	if got := triviaKinds(one.Trailing); !slices.Equal(got, trailing) {
		t.Errorf("trailing of 1 = %v, want %v", got, trailing)
	}
	if got := triviaKinds(b.Leading); !slices.Equal(got, leading) {
		t.Errorf("leading of b = %v, want %v", got, leading)
	}
	if nl.Kind != syntax.TokNL || nl.Start != nl.End || nl.Start != one.Trailing[len(one.Trailing)-1].End {
		t.Errorf("NL token %+v is not empty at the trailing border", nl)
	}
	if f.Tokens[0].Kind != syntax.TokBOF || f.First() != syntax.NoTok+1 {
		t.Errorf("Tokens[0] is %s, the file starts at %d", f.Tokens[0].Kind, f.First())
	}
}

func triviaKinds(ts []syntax.Trivia) []syntax.TriviaKind {
	out := make([]syntax.TriviaKind, len(ts))
	for i, tr := range ts {
		out[i] = tr.Kind
	}
	return out
}

// FORMATTER.md §8.1: an item's trailing comment after its separator comma is its own.
func TestTrailingAfterComma(t *testing.T) {
	f, _, _ := parseText(t, "a.canon", []byte("package a\n\nlet t = {\n  a: 1, // one\n  b: 2\n}\n"))
	lit := f.Decls[0].(*syntax.LetDecl).Value.(*syntax.BraceLit)
	tr := f.Trailing(lit.Items[0])
	if len(tr) == 0 || tr[len(tr)-1].Kind != syntax.TriviaLineComment {
		t.Errorf("Trailing(a: 1) = %v, want the comment after the comma", triviaKinds(tr))
	}
	if got := f.Trailing(lit.Items[1]); len(got) != 0 {
		t.Errorf("Trailing(b: 2) = %v, want none", triviaKinds(got))
	}
}

// GRAMMAR.md §4.3: LookupWord and IsNameable classify words for printers.
func TestLookupWord(t *testing.T) {
	tests := []struct {
		word     string
		kind     syntax.TokenKind
		nameable bool
	}{
		{"check", syntax.KwCheck, true},
		{"if", syntax.KwIf, false},
		{"_", syntax.TokUnderscore, false},
		{"heal", syntax.TokIdent, false},
		{"package", syntax.KwPackage, true},
	}
	for _, tt := range tests {
		k := syntax.LookupWord(tt.word)
		if k != tt.kind || syntax.IsNameable(k) != tt.nameable {
			t.Errorf("LookupWord(%q) = %s nameable %v", tt.word, k, syntax.IsNameable(k))
		}
	}
}
