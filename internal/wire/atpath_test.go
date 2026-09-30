package wire_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/wire"
)

// atSeeds are a few of WIRE.md §6.3's own path shapes, well-formed or not.
var atSeeds = []string{
	"", "a", "a.b", "a.b.c", "*", "a.*", "[0]", "a[0]", "a[10].b", `a\.b`, `a\[0\]`, "a.", ".", "[", "[0", "a[",
}

// WIRE.md 6.3: ParseAt reads names (with `\` escapes), `*` and `[n]`, and refuses an empty or
// malformed path.
func TestParseAt(t *testing.T) {
	name := func(s string) wire.AtStep { return wire.AtStep{Kind: wire.AtName, Name: s} }
	index := func(n int) wire.AtStep { return wire.AtStep{Kind: wire.AtIndex, Index: n} }
	star := wire.AtStep{Kind: wire.AtStar}
	for _, c := range []struct {
		path string
		want []wire.AtStep
		ok   bool
	}{
		{"a.b", []wire.AtStep{name("a"), name("b")}, true},
		{"*.us", []wire.AtStep{star, name("us")}, true},
		{"a[10].*", []wire.AtStep{name("a"), index(10), star}, true},
		{"[0]", []wire.AtStep{index(0)}, true},
		{`a\.b`, []wire.AtStep{name("a.b")}, true},
		{"", nil, false},
		{"a.", nil, false},
		{"[01]", nil, false},
		{`a\x`, nil, false},
		{"a*", nil, false},
	} {
		got, ok := wire.ParseAt(c.path)
		if ok != c.ok || !slices.Equal(got, c.want) {
			t.Errorf("ParseAt(%q) = %v %v, want %v %v", c.path, got, ok, c.want, c.ok)
		}
	}
}

// IMPLEMENTATION-PLAN.md §7.7: the `at:` path parser never panics, on any input, well-formed or not.
func FuzzParseAt(f *testing.F) {
	for _, s := range atSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, path string) {
		wire.ParseAt(path)
	})
}
