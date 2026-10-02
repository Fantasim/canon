package lsp

import (
	"runtime"
	"testing"

	"github.com/fantasim/canonlang/internal/source"
)

// IMPLEMENTATION-PLAN §8.4 Positions, CLI.md §2.4: UTF-8 byte columns to UTF-16 characters.
func TestUTF16Columns(t *testing.T) {
	text := []byte("ab\né = 1\n中文 x\n😀y\n\xffz\nend")
	l := newLines(text)
	cases := []struct {
		name      string
		line, col int
		want      position
	}{
		{"ASCII start", 1, 1, position{0, 0}},
		{"ASCII end, exclusive", 1, 3, position{0, 2}},
		{"after an accent (2 bytes, 1 unit)", 2, 3, position{1, 1}},
		{"after two CJK characters (3 bytes, 1 unit each)", 3, 7, position{2, 2}},
		{"a column after CJK text", 3, 8, position{2, 3}},
		{"after an emoji (4 bytes, a surrogate pair)", 4, 5, position{3, 2}},
		{"a column after the emoji", 4, 6, position{3, 3}},
		{"after an invalid byte (one U+FFFD unit)", 5, 2, position{4, 1}},
		{"past the line end: excess bytes kept", 2, 12, position{1, 10}},
		{"last line without a newline", 6, 4, position{5, 3}},
		{"no line", 0, 0, position{}},
		{"line past the end", 9, 1, position{}},
	}
	for _, tc := range cases {
		if got := l.at(tc.line, tc.col); got != tc.want {
			t.Errorf("%s: at(%d, %d) = %+v, want %+v", tc.name, tc.line, tc.col, got, tc.want)
		}
	}
	span := l.span(source.Location{Line: 4, Col: 1, EndLine: 4, EndCol: 6})
	if want := (textRange{Start: position{3, 0}, End: position{3, 3}}); span != want {
		t.Errorf("span = %+v, want %+v", span, want)
	}
}

// IMPLEMENTATION-PLAN §8.4 Positions: UTF-16 lengths of whole texts.
func TestUTF16Len(t *testing.T) {
	cases := map[string]int{"": 0, "abc": 3, "é": 1, "中": 1, "😀": 2, "é中😀x": 5, "\xff\xfe": 2}
	for text, want := range cases {
		if got := utf16Len([]byte(text)); got != want {
			t.Errorf("utf16Len(%q) = %d, want %d", text, got, want)
		}
	}
}

// IMPLEMENTATION-PLAN §8.4 Transport: file URIs and absolute names convert both ways.
func TestURIs(t *testing.T) {
	names := []string{"/home/a b/x.canon", "/tmp/100%/a#b?.json", "/d/é中😀.canon"}
	if runtime.GOOS == "windows" {
		names = []string{"C:/Users/a b/x.canon", "//server/share/x.canon"}
	}
	for _, name := range names {
		got, ok := pathOf(uriOf(name))
		if !ok || got != name {
			t.Errorf("pathOf(uriOf(%q)) = %q, %v", name, got, ok)
		}
	}
	if got := uriOf("/home/a b/x.canon"); runtime.GOOS != "windows" && got != "file:///home/a%20b/x.canon" {
		t.Errorf("uriOf = %q", got)
	}
	for _, uri := range []string{"untitled:Untitled-1", "https://example.com/x.canon", "file://", "%zz"} {
		if got, ok := pathOf(uri); ok {
			t.Errorf("pathOf(%q) = %q, want no file", uri, got)
		}
	}
}
