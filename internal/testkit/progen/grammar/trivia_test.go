package grammar_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
	"github.com/fantasim/canonlang/internal/testkit/progen/grammar"
)

const examplesDir = "../../../../examples"

// IMPLEMENTATION-PLAN §4.1, FMT-01, API-03, DECISIONS 73: trivia read back is the lexer's.
func TestTriviaReadBack(t *testing.T) {
	err := filepath.WalkDir(examplesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".canon" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, _ := grammar.Parse(filepath.Base(path), data)
		checkTrivia(t, path, f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range grammar.Kinds() {
		for seed := range uint64(seeds) {
			r := progen.NewRand(seed)
			src := grammar.GenerateKind(r, progen.NewBudget(120, 6), k)
			f, _ := grammar.ParseKind(k.File(), src, k)
			checkTrivia(t, k.String(), f)
			f, _ = grammar.ParseKind(k.File(), grammar.Corrupt(r, src), k)
			checkTrivia(t, k.String()+" corrupted", f)
		}
	}
}

// checkTrivia checks f's trivia: leading trivia, token and trailing trivia print the file back;
// trailing trivia stops at the first line break, leading trivia starts at one (any before the
// first token); NL tokens have none; every trivia is the kind its bytes say.
func checkTrivia(t *testing.T, name string, f *syntax.File) {
	t.Helper()
	src := f.Src.Content
	var out []byte
	for i, tok := range f.Tokens {
		lead, trail := f.AppendLeading(nil, syntax.Tok(i)), f.AppendTrailing(nil, syntax.Tok(i))
		if i > 1 && len(lead) > 0 && lead[0].Kind != syntax.TriviaNewline {
			t.Fatalf("%s: token %d's leading trivia starts with %d, not a line break", name, i, lead[0].Kind)
		}
		if tok.Kind == syntax.TokNL && (len(lead) > 0 || len(trail) > 0) {
			t.Fatalf("%s: NL token %d has trivia", name, i)
		}
		for _, tr := range lead {
			out = append(out, triviaText(t, name, src, tr)...)
		}
		out = append(out, src[tok.Start:tok.End]...)
		for _, tr := range trail {
			if tr.Kind == syntax.TriviaNewline {
				t.Fatalf("%s: token %d's trailing trivia holds a line break", name, i)
			}
			out = append(out, triviaText(t, name, src, tr)...)
		}
	}
	if !bytes.Equal(out, src) {
		t.Fatalf("%s: tokens and trivia do not reproduce the file", name)
	}
}

// triviaText is tr's bytes, checked against its kind; "///" is a doc line after blanks only.
func triviaText(t *testing.T, name string, src []byte, tr syntax.Trivia) []byte {
	t.Helper()
	text := src[tr.Start:tr.End]
	lineStart := bytes.LastIndexByte(src[:tr.Start], '\n') + 1
	before := bytes.TrimPrefix(src[lineStart:tr.Start], []byte("\xEF\xBB\xBF"))
	doc := bytes.HasPrefix(text, []byte("///")) && !bytes.HasPrefix(text, []byte("////")) &&
		len(bytes.Trim(before, " \t\r")) == 0
	ok := map[syntax.TriviaKind]bool{
		syntax.TriviaSpace:        len(bytes.Trim(text, " \t")) == 0 || string(text) == "\r",
		syntax.TriviaNewline:      string(text) == "\n",
		syntax.TriviaLineComment:  bytes.HasPrefix(text, []byte("//")) && !doc && !bytes.Contains(text, []byte("\n")),
		syntax.TriviaDocComment:   doc && !bytes.Contains(text, []byte("\n")),
		syntax.TriviaBlockComment: bytes.HasPrefix(text, []byte("/*")),
		syntax.TriviaBOM:          tr.Start == 0 && string(text) == "\xEF\xBB\xBF",
	}[tr.Kind]
	if !ok || len(text) == 0 {
		t.Fatalf("%s: trivia %d at %d is %q", name, tr.Kind, tr.Start, text)
	}
	return text
}
