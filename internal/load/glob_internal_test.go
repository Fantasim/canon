package load

import "testing"

// WIRE.md §6.5's "Base": the segments before the first magic character are the literal prefix.
func TestSplitGlob(t *testing.T) {
	cases := []struct {
		pattern, prefix, rest string
	}{
		{"data/a.json", "data/a.json", ""},
		{"data/*.json", "data/", "*.json"},
		{"*.json", "", "*.json"},
		{"data/sub/**/x.json", "data/sub/", "**/x.json"},
		{"a[b]/c", "", "a[b]/c"},
	}
	for _, c := range cases {
		prefix, rest := splitGlob(c.pattern)
		if prefix != c.prefix || rest != c.rest {
			t.Errorf("splitGlob(%q) = %q, %q; want %q, %q", c.pattern, prefix, rest, c.prefix, c.rest)
		}
	}
}

// validateGlob's verdict on a glob rest: E7001, E7005, a directory reference, or well-formed.
func TestValidateGlob(t *testing.T) {
	cases := []struct {
		rest string
		want globCheck
	}{
		{"", globCheck{}},
		{"*.json", globCheck{}},
		{"a/", globCheck{dirOnly: true}},
		{"a/**", globCheck{}},
		{"a/**b", globCheck{e7005Cause: causeDoubleStar}},
		{"a/[abc", globCheck{e7005Cause: causeBracket}},
		{"a/{b,c", globCheck{e7005Cause: causeBrace}},
		{"a/{b,}", globCheck{e7005Cause: causeBrace}},
		{"a/{{b}}", globCheck{e7005Cause: causeBrace}},
		{"a//b", globCheck{e7001: true}},
		{"a/./b", globCheck{e7001: true}},
		{"a/../b", globCheck{e7001: true}},
		{"a/[!x]", globCheck{}},
		{"a/{b,c}", globCheck{}},
		{"a/{a,[}]}", globCheck{}},
		{"a/{a,[,,]}", globCheck{}},
		{"a/{a,[b}", globCheck{e7005Cause: causeBrace}},
		{"a/{[]}", globCheck{e7005Cause: causeBrace}},
	}
	for _, c := range cases {
		if got := validateGlob(c.rest); got != c.want {
			t.Errorf("validateGlob(%q) = %+v, want %+v", c.rest, got, c.want)
		}
	}
}

// Only "!" negates a class; "^" and a leading "]" are literal, so "[]" alone never closes.
func TestSkipClass(t *testing.T) {
	cases := []struct {
		seg  string
		end  int
		want bool
	}{
		{"[]", 0, false},
		{"[]a]", 4, true},
		{"[!]a]", 5, true},
		{"[^abc]", 6, true},
		{"[a-z]", 5, true},
	}
	for _, c := range cases {
		end, ok := skipClass(c.seg, 0)
		if ok != c.want || (ok && end != c.end) {
			t.Errorf("skipClass(%q) = %d, %v; want %d, %v", c.seg, end, ok, c.end, c.want)
		}
	}
}

// WIRE.md §6.5: nested or empty-alternative braces fail; a class inside is skipped whole (R2-8).
func TestSkipBrace(t *testing.T) {
	cases := []struct {
		seg  string
		end  int
		want bool
	}{
		{"{a,b}", 5, true},
		{"{a,{b}}", 0, false},
		{"{a,}", 0, false},
		{"{,a}", 0, false},
		{"{a,[}]}", 7, true},
		{"{a,[,,]}", 8, true},
		{"{[!}]x,y}", 9, true},
		{"{a,[b}", 0, false},
	}
	for _, c := range cases {
		end, ok := skipBrace(c.seg, 0)
		if ok != c.want || (ok && end != c.end) {
			t.Errorf("skipBrace(%q) = %d, %v; want %d, %v", c.seg, end, ok, c.end, c.want)
		}
	}
}

// WIRE.md §6.5: a "}" or "," inside a class inside braces is a literal of that class.
func TestSegMatchesClassInBraces(t *testing.T) {
	cases := []struct {
		seg, name string
		want      bool
	}{
		{"{a,[}]}", "}", true},
		{"{a,[}]}", "a", true},
		{"{a,[}]}", "b", false},
		{"{a,[,,]}", ",", true},
		{"x{[]],y}", "x]", true},
	}
	for _, c := range cases {
		if got := segMatches(c.seg, c.name); got != c.want {
			t.Errorf("segMatches(%q, %q) = %v, want %v", c.seg, c.name, got, c.want)
		}
	}
}
