package format_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// FORMATTER.md §2 and §10: no BOM, "\n" line ends, a final "\n", spaces, no trailing blank.
func TestCharactersAndLines(t *testing.T) {
	cases := []struct{ rule, in, want string }{
		{"§10 BOM", "\xEF\xBB\xBFpackage p\n\nconst X = 1\n", "package p\n\nconst X = 1\n"},
		{"§10 CRLF", "package p\r\n\r\nconst X = 1\r\n", "package p\n\nconst X = 1\n"},
		{"§10 tabs", "package p\n\nrecord R {\n\ta: Int\n\t\tb: Int\n}\n", "package p\n\nrecord R {\n  a: Int\n  b: Int\n}\n"},
		{"§2 final line break", "package p\n\nconst X = 1", "package p\n\nconst X = 1\n"},
		{"§2 trailing blanks", "package p   \n\nconst X = 1 \t\n", "package p\n\nconst X = 1\n"},
		{"§2 trailing blanks of a comment", "package p\n\n// c \t\nconst X = 1\n", "package p\n\n// c\nconst X = 1\n"},
		{"§2 §8.2 block comment lines", "package p\n\n/* a \n  b\t\n */\nconst X = 1\n", "package p\n\n/* a\n  b\n */\nconst X = 1\n"},
		{"§4 end of file", "package p\n\nconst X = 1\n\n\n", "package p\n\nconst X = 1\n"},
	}
	for _, c := range cases {
		got, err := formatText(t, "a/a.canon", []byte(c.in))
		if err != nil || string(got) != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.rule, got, err, c.want)
		}
	}
}

// FORMATTER.md §2: the width is counted in code points, not bytes.
func TestWidthCountsCodePoints(t *testing.T) {
	head := `let x = f("`
	fits := head + strings.Repeat("é", format.Width-len(head)-len(`")`)) + `")`
	over := head + strings.Repeat("é", format.Width-len(head)-len(`")`)+1) + `")`
	for _, c := range []struct {
		line  string
		lines int
	}{{fits, 1}, {over, 2}} {
		got, err := formatText(t, "a/a.canon", []byte("package p\n\n"+c.line+"\n"))
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(string(got), "\n") - 2; n != c.lines {
			t.Errorf("%d code points: %d lines, want %d:\n%s", len([]rune(c.line)), n, c.lines, got)
		}
	}
}

// FORMATTER.md §1 and §16: a syntax error is refused, a lone byte order mark is not one.
func TestSyntaxErrorsAreRefused(t *testing.T) {
	for _, in := range []string{"package p\n\nconst = 1\n", "const X = 1\n", "package p\n\nlet x = \"open\n"} {
		if out, err := formatText(t, "a/a.canon", []byte(in)); !errors.Is(err, format.ErrSyntax) || out != nil {
			t.Errorf("%q: got %q, %v; want ErrSyntax", in, out, err)
		}
	}
	for _, in := range []string{"package p\n\nconst = 1\n", "const X = 1\n", "package p\n\nrecord R { a: }\n"} {
		if out, err := format.File(parse(t, "a/a.canon", []byte(in)).file); !errors.Is(err, format.ErrSyntax) || out != nil {
			t.Errorf("File %q: got %q, %v; want ErrSyntax", in, out, err)
		}
	}
	if _, err := formatText(t, "a/a.canon", []byte("\xEF\xBB\xBFpackage p\n")); err != nil {
		t.Errorf("a byte order mark alone: %v", err)
	}
}

// FORMATTER.md §4: at most one blank line anywhere, including between comments.
func TestAtMostOneBlankLine(t *testing.T) {
	got, err := formatText(t, "a/a.canon", []byte("package p\n\n\n\n// a\n\n\n\n// b\n\n\n\nconst X = 1\n"))
	if err != nil || strings.Contains(string(got), "\n\n\n") {
		t.Errorf("got %q, %v", got, err)
	}
}

// FORMATTER.md §4, §8.1, DECISIONS 211: a doc block (GRAMMAR.md §2.2) stays apart from another.
func TestBlankLineBetweenLeadingCommentsIsKept(t *testing.T) {
	cases, err := golden.Load("testdata/fmt/doc_block_blanks.txtar")
	if err != nil {
		t.Fatal(err)
	}
	in := cases[0].Archive.Files[0]
	got, err := formatText(t, in.Name, in.Data)
	if err != nil {
		t.Fatal(err)
	}
	want, err := cases[0].Want()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	checkFormatted(t, in.Name, parse(t, in.Name, in.Data), got)
}
