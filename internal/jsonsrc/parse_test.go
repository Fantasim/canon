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

// WIRE.md §3.1, DECISIONS 162: anything else is one E7109 at the offending character.
func TestParseRejects(t *testing.T) {
	tests := []struct {
		in     string
		at     int
		detail string
	}{
		{"", 0, `""`},
		{"   ", 3, `""`},
		{`{"a": 1,}`, 8, `"}"`},
		{"[1,]", 3, `"]"`},
		{"[1 2]", 3, `"2"`},
		{`{"a" 1}`, 5, `"1"`},
		{"{a: 1}", 1, `"a"`},
		{"{'a': 1}", 1, `"'"`},
		{"// c\n{}", 0, `"/"`},
		{"{} /* c */", 3, `"/"`},
		{"NaN", 0, `"N"`},
		{"Infinity", 0, `"I"`},
		{"-Infinity", 1, `"I"`},
		{"+1", 0, `"+"`},
		{"-", 1, `""`},
		{"-a", 1, `"a"`},
		{"01", 1, `"1"`},
		{"-01", 2, `"1"`},
		{".5", 0, `"."`},
		{"1.", 2, `""`},
		{"1.e5", 2, `"e"`},
		{"1e", 2, `""`},
		{"1e+", 3, `""`},
		{"tru", 3, `""`},
		{"trUe", 2, `"U"`},
		{"nul l", 3, `" "`},
		{"\"a\tb\"", 2, `"\t"`},
		{"\"a\nb\"", 2, `"\n"`},
		{`"\x"`, 2, `"x"`},
		{`"\U0041"`, 2, `"U"`},
		{`"\u12G4"`, 5, `"G"`},
		{`"\u12`, 5, `""`},
		{`"\`, 2, `""`},
		{`"abc`, 4, `""`},
		{"{} {}", 3, `"{"`},
		{"[1] x", 4, `"x"`},
		{`"é" ü`, 5, `"ü"`},
		{"[1,\xef\xbb\xbf2]", 3, "\"\ufeff\""},
		{"{\"a\":1}\x00", 7, `"\u0000"`},
		{`{"a":1 "b":2}`, 7, `"\""`},
		{`[,]`, 1, `","`},
		{`{"a":}`, 5, `"}"`},
	}
	for _, tt := range tests {
		p := parse(t, tt.in)
		if !errors.Is(p.err, jsonsrc.ErrSyntax) || p.root != nil {
			t.Errorf("%q: %v, want ErrSyntax and no tree", tt.in, p.err)
			continue
		}
		wantSyntax(t, p, tt.at, tt.detail)
	}
}

// wantSyntax checks that p holds exactly one E7109, at byte at, over one character.
func wantSyntax(t *testing.T, p parsed, at int, detail string) {
	t.Helper()
	if len(p.findings) != 1 || p.findings[0].Code != diag.E7109.Def().Code {
		t.Errorf("%q: findings %v, want one syntax error", p.file.Content, p.findings)
		return
	}
	f := p.findings[0]
	width := len(strings.TrimSuffix(strings.TrimPrefix(detail, `"`), `"`))
	if strings.HasPrefix(detail, `"\`) {
		width = 1
	}
	if int(f.Span.Start) != at || f.Span.Len() != width || !strings.HasSuffix(f.Message, ": "+detail) {
		t.Errorf("%q: %s at %d+%d, want %s at %d+%d", p.file.Content, f.Message, f.Span.Start, f.Span.Len(), detail, at, width)
	}
}

// WIRE.md §3.1: 512 nested arrays or objects are read; the 513th opening byte is E7109.
func TestParseDepth(t *testing.T) {
	const limit = 512
	for _, open := range []string{"[", `{"a":`} {
		closing := map[string]string{"[": "]", `{"a":`: "}"}[open]
		deep := strings.Repeat(open, limit) + "0" + strings.Repeat(closing, limit)
		mustParse(t, deep)
		p := parse(t, strings.Repeat(open, limit+1)+"0"+strings.Repeat(closing, limit+1))
		wantSyntax(t, p, limit*len(open), `"`+open[:1]+`"`)
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
