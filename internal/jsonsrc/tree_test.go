package jsonsrc_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
)

// WIRE.md §3.2, DECISIONS 164: one E7104 per repeated key, only when the source parses.
func TestDuplicateKeys(t *testing.T) {
	tests := []struct {
		in       string
		at       []int
		pointers []string
	}{
		{`{"a": 1, "\u0061": 2}`, []int{9}, []string{"/a"}},
		{`{"a": {"a": 1}, "b": [{"x/": 1, "x/": 2, "x/": 3}]}`, []int{32, 41}, []string{"/b/0/x~1", "/b/0/x~1"}},
		{`{"a": 1, "b": {"a": 1}, "c": [{"a": 1}, {"a": 1}]}`, nil, nil},
	}
	for _, tt := range tests {
		p := parse(t, tt.in)
		var at []int
		var pointers []string
		for _, f := range p.findings {
			if f.Code != diag.E7104.Def().Code {
				t.Errorf("%q: %s, want a duplicate key", tt.in, f.Code)
			}
			at = append(at, int(f.Span.Start))
			pointers = append(pointers, f.Pointer)
		}
		if !slices.Equal(at, tt.at) || !slices.Equal(pointers, tt.pointers) {
			t.Errorf("%q: repeats at %v %v, want %v %v", tt.in, at, pointers, tt.at, tt.pointers)
		}
		if wantErr := len(tt.at) > 0; wantErr != errors.Is(p.err, jsonsrc.ErrDuplicateKey) || wantErr != (p.root == nil) {
			t.Errorf("%q: %v, tree %v", tt.in, p.err, p.root != nil)
		}
	}
	var wide strings.Builder
	for i := range 20 {
		fmt.Fprintf(&wide, "\"k%d\": %d,\n", i%18, i)
	}
	p := parse(t, "{\n"+wide.String()+`"k3": 0}`)
	if len(p.findings) != 3 || p.findings[2].Pointer != "/k3" || !strings.HasSuffix(p.findings[2].Message, "a.json:5)") {
		t.Errorf("a wide object: %v, want repeats of k0, k1 and k3", p.findings)
	}
	p = parse(t, `{"a": 1, "a": 2,}`)
	if !errors.Is(p.err, jsonsrc.ErrSyntax) || len(p.findings) != 1 || p.findings[0].Code != diag.E7109.Def().Code {
		t.Errorf("a repeat before a syntax error: %v %v, want only the syntax error", p.err, p.findings)
	}
}

// WIRE.md §3.1, §3.2, DECISIONS 163: what load reports as E7105 is an *EncodingError.
func TestEncoding(t *testing.T) {
	tests := []struct {
		in         string
		start, end source.Pos
		surrogate  rune
	}{
		{"\xff", 0, 1, 0},
		{"{\"a\": \"\xc3\"}", 7, 8, 0},
		{"{,\"\xe9\"}", 3, 4, 0},
		{"\xfe\xff\x00{", 0, 2, 0},
		{"\xff\xfe{\x00", 0, 2, 0},
		{"\x00\x00\xfe\xff", 0, 4, 0},
		{"\xff\xfe\x00\x00", 0, 4, 0},
		{`"\ud800"`, 1, 7, 0xd800},
		{`"\uDC00"`, 1, 7, 0xdc00},
		{`"\ud800\u0041"`, 1, 7, 0xd800},
		{`"\ud800\ud800"`, 1, 7, 0xd800},
		{`"\ud800\x"`, 1, 7, 0xd800},
		{`"\ud800`, 1, 7, 0xd800},
		{`"\ud800\udcGG"`, 1, 7, 0xd800},
		{`"\ud800\udc`, 1, 7, 0xd800},
		{`["\ud83d\ude00", "\ud83d"]`, 18, 24, 0xd83d},
	}
	for _, tt := range tests {
		p := parse(t, tt.in)
		var enc *jsonsrc.EncodingError
		if !errors.As(p.err, &enc) || !errors.Is(p.err, jsonsrc.ErrEncoding) {
			t.Errorf("%q: %v, want an EncodingError", tt.in, p.err)
			continue
		}
		if enc.Span.Start != tt.start || enc.Span.End != tt.end || enc.Surrogate != tt.surrogate || len(p.findings) > 0 {
			t.Errorf("%q: %+v %v, want %d-%d %U and no finding", tt.in, *enc, p.findings, tt.start, tt.end, tt.surrogate)
		}
		if !strings.Contains(enc.Error(), jsonsrc.ErrEncoding.Error()) {
			t.Errorf("%q: %q", tt.in, enc.Error())
		}
	}
}

// RFC 6901 §3, §4: "~" is "~0" and "/" is "~1" in a key; an element's token is its index.
func TestPointers(t *testing.T) {
	p := mustParse(t, `{"a/b": {"m~n": [0, {"": 1, "~1": 2, "/": 3, "é": 4}]}}`)
	var got []string
	walk(p.root, func(n *jsonsrc.Node) { got = append(got, n.Pointer()) })
	want := []string{
		"", "/a~1b", "/a~1b/m~0n", "/a~1b/m~0n/0", "/a~1b/m~0n/1",
		"/a~1b/m~0n/1/", "/a~1b/m~0n/1/~01", "/a~1b/m~0n/1/~1", "/a~1b/m~0n/1/é",
	}
	if !slices.Equal(got, want) {
		t.Errorf("pointers\n got %q\nwant %q", got, want)
	}
}

// EVALUATION.md §13: a value spans its bytes, a key its token; columns count UTF-8 bytes.
func TestSpans(t *testing.T) {
	p := mustParse(t, "{\"é\": \"日本\",\n  \"n\": [-1.5e3, true, null, {}]}")
	var got []string
	walk(p.root, func(n *jsonsrc.Node) { got = append(got, p.text(n.Span)) })
	want := []string{
		p.text(source.Span{End: source.Pos(len(p.file.Content))}), `"日本"`,
		"[-1.5e3, true, null, {}]", "-1.5e3", "true", "null", "{}",
	}
	if !slices.Equal(got, want) {
		t.Errorf("spans\n got %q\nwant %q", got, want)
	}
	keys := []string{p.text(p.root.Members[0].KeySpan), p.text(p.root.Members[1].KeySpan)}
	if !slices.Equal(keys, []string{`"é"`, `"n"`}) {
		t.Errorf("key spans %q", keys)
	}
	value := p.fs.Locate(p.root.Members[0].Value.Span)
	elem := p.fs.Locate(p.root.Members[1].Value.Elems[1].Span)
	if value.Line != 1 || value.Col != 8 || value.EndCol != 16 || elem.Line != 2 || elem.Col != 17 {
		t.Errorf("locations %+v %+v", value, elem)
	}
	bom := mustParse(t, "\xef\xbb\xbf{\"a\": 1}")
	if s := bom.root.Members[0].Value.Span; s.Start != 9 || s.End != 10 || bom.root.Span.Start != 3 {
		t.Errorf("after a BOM: %+v, root %+v", s, bom.root.Span)
	}
}

// DECISIONS 162: spans are in the content FileSet.Add normalized, so "\r\n" counts one byte.
func TestLocationsAfterCRLF(t *testing.T) {
	tests := []struct {
		in        string
		line, col int
		message   string
	}{
		{"{\r\n  \"a\": 1,\r\n  \"a\": 2\r\n}\r\n", 3, 3, "a.json:2)"},
		{"[\r\n  1,\r\n]\r\n", 3, 1, `"]"`},
		{"[\"a\r\nb\"]", 1, 4, `"\n"`},
	}
	for _, tt := range tests {
		p := parse(t, tt.in)
		if len(p.findings) != 1 {
			t.Errorf("%q: %v, want one finding", tt.in, p.findings)
			continue
		}
		loc := p.fs.Locate(p.findings[0].Span)
		if loc.Line != tt.line || loc.Col != tt.col || !strings.HasSuffix(p.findings[0].Message, tt.message) {
			t.Errorf("%q: %s at %d:%d, want %s at %d:%d", tt.in, p.findings[0].Message, loc.Line, loc.Col, tt.message, tt.line, tt.col)
		}
	}
}
