package jsonsrc_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
)

// WIRE.md §3.1-§3.3: RFC 8259 values, a leading BOM, exact number text, decoded strings.
func TestParseAccepts(t *testing.T) {
	tests := []struct {
		in   string
		kind jsonsrc.Kind
		text string
	}{
		{"null", jsonsrc.Null, "null"},
		{" \t\r\n true \n", jsonsrc.Bool, "true"},
		{"false", jsonsrc.Bool, "false"},
		{"-0", jsonsrc.Number, "-0"},
		{"0.5e-3", jsonsrc.Number, "0.5e-3"},
		{"1E+5", jsonsrc.Number, "1E+5"},
		{"-12.50", jsonsrc.Number, "-12.50"},
		{"1e999999999", jsonsrc.Number, "1e999999999"},
		{"123456789012345678901234567890.000000000000000000001", jsonsrc.Number, "123456789012345678901234567890.000000000000000000001"},
		{`"\u0000"`, jsonsrc.String, "\x00"},
		{`"\"\\\/\b\f\n\r\t"`, jsonsrc.String, "\"\\/\b\f\n\r\t"},
		{`"\ud83d\ude00\u00E9"`, jsonsrc.String, "😀é"},
		{`"é日本 <>&"`, jsonsrc.String, "é日本 <>&"},
		{"\xef\xbb\xbf{}", jsonsrc.Object, ""},
		{"[]", jsonsrc.Array, ""},
	}
	for _, tt := range tests {
		p := mustParse(t, tt.in)
		if p.root.Kind != tt.kind || p.root.Text != tt.text {
			t.Errorf("%q: kind %d text %q, want %d %q", tt.in, p.root.Kind, p.root.Text, tt.kind, tt.text)
		}
	}
}

// WIRE.md §3.1, DECISIONS 208: anything else is one E7109, eof, depth or char.
func TestParseRejects(t *testing.T) {
	tests := []struct {
		in     string
		at     int
		width  int
		detail string
	}{
		{"", 0, 0, "unexpected end of input"},
		{"   ", 3, 0, "unexpected end of input"},
		{`{"a": 1,}`, 8, 1, `unexpected character "}"`},
		{"[1,]", 3, 1, `unexpected character "]"`},
		{"[1 2]", 3, 1, `unexpected character "2"`},
		{`{"a" 1}`, 5, 1, `unexpected character "1"`},
		{"{a: 1}", 1, 1, `unexpected character "a"`},
		{"{'a': 1}", 1, 1, `unexpected character "'"`},
		{"// c\n{}", 0, 1, `unexpected character "/"`},
		{"{} /* c */", 3, 1, `unexpected character "/"`},
		{"NaN", 0, 1, `unexpected character "N"`},
		{"Infinity", 0, 1, `unexpected character "I"`},
		{"-Infinity", 1, 1, `unexpected character "I"`},
		{"+1", 0, 1, `unexpected character "+"`},
		{"-", 1, 0, "unexpected end of input"},
		{"-a", 1, 1, `unexpected character "a"`},
		{"01", 1, 1, `unexpected character "1"`},
		{"-01", 2, 1, `unexpected character "1"`},
		{".5", 0, 1, `unexpected character "."`},
		{"1.", 2, 0, "unexpected end of input"},
		{"1.e5", 2, 1, `unexpected character "e"`},
		{"1e", 2, 0, "unexpected end of input"},
		{"1e+", 3, 0, "unexpected end of input"},
		{"tru", 3, 0, "unexpected end of input"},
		{"trUe", 2, 1, `unexpected character "U"`},
		{"nul l", 3, 1, `unexpected character " "`},
		{"\"a\tb\"", 2, 1, "unexpected character \"\\t\""},
		{"\"a\nb\"", 2, 1, "unexpected character \"\\n\""},
		{`"\x"`, 2, 1, `unexpected character "x"`},
		{`"\U0041"`, 2, 1, `unexpected character "U"`},
		{`"\u12G4"`, 5, 1, `unexpected character "G"`},
		{`"\u12`, 5, 0, "unexpected end of input"},
		{`"\`, 2, 0, "unexpected end of input"},
		{`"abc`, 4, 0, "unexpected end of input"},
		{"{} {}", 3, 1, `unexpected character "{"`},
		{"[1] x", 4, 1, `unexpected character "x"`},
		{`"é" ü`, 5, 2, `unexpected character "ü"`},
		{"[1,\xef\xbb\xbf2]", 3, 3, "unexpected character \"\ufeff\""},
		{"{\"a\":1}\x00", 7, 1, "unexpected character \"\\u0000\""},
		{`{"a":1 "b":2}`, 7, 1, "unexpected character \"\\\"\""},
		{`[,]`, 1, 1, `unexpected character ","`},
		{`{"a":}`, 5, 1, `unexpected character "}"`},
	}
	for _, tt := range tests {
		p := parse(t, tt.in)
		if !errors.Is(p.err, jsonsrc.ErrSyntax) || p.root != nil {
			t.Errorf("%q: %v, want ErrSyntax and no tree", tt.in, p.err)
			continue
		}
		wantSyntax(t, p, tt.at, tt.width, tt.detail)
	}
}

// wantSyntax checks that p holds one E7109 at byte at over width bytes (WIRE.md §3.1, DECISIONS 208).
func wantSyntax(t *testing.T, p parsed, at, width int, detail string) {
	t.Helper()
	if len(p.findings) != 1 || p.findings[0].Code != diag.E7109.Def().Code {
		t.Errorf("%q: findings %v, want one syntax error", p.file.Content, p.findings)
		return
	}
	f := p.findings[0]
	if int(f.Span.Start) != at || f.Span.Len() != width || !strings.HasSuffix(f.Message, ": "+detail) {
		t.Errorf("%q: %s at %d+%d, want %q at %d+%d", p.file.Content, f.Message, f.Span.Start, f.Span.Len(), detail, at, width)
	}
}

// WIRE.md §3.1, DECISIONS 208: the 513th opener past the 512 limit is the E7109 depth variant.
func TestParseDepth(t *testing.T) {
	const limit = 512
	for _, open := range []string{"[", `{"a":`} {
		closing := map[string]string{"[": "]", `{"a":`: "}"}[open]
		deep := strings.Repeat(open, limit) + "0" + strings.Repeat(closing, limit)
		mustParse(t, deep)
		p := parse(t, strings.Repeat(open, limit+1)+"0"+strings.Repeat(closing, limit+1))
		wantSyntax(t, p, limit*len(open), 1, "nested deeper than 512 levels")
	}
}

// API.md §4.1 (Pointer): an E7109 names the innermost array or object it lies in.
func TestSyntaxPointer(t *testing.T) {
	tests := []struct{ in, pointer string }{
		{`{"a": [1, 2,]}`, "/a"},
		{`{"a": {"b/c" 1}}`, "/a"},
		{`{"a": {"b/c": [x]}}`, "/a/b~1c"},
		{`{"a": [], "b": [{}, {"c": x}]}`, "/b/1"},
		{`[[], [1] 2]`, ""},
		{"1 2", ""},
	}
	for _, tt := range tests {
		p := parse(t, tt.in)
		if len(p.findings) != 1 || p.findings[0].Pointer != tt.pointer {
			t.Errorf("%q: %v, want one finding at pointer %q", tt.in, p.findings, tt.pointer)
		}
	}
}
