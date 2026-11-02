package load_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/load"
)

// CODEGEN.md §2.4, DECISIONS 451, WIRE.md §6.5: MatchesPlanned applies the walk's segment rules.
func TestMatchesPlanned(t *testing.T) {
	cases := []struct {
		name, pattern, display string
		want                   bool
	}{
		{"star", "a/data/*.json", "a/data/y.json", true},
		{"star misses the extension", "a/data/*.csv", "a/data/y.json", false},
		{"star stays in its directory", "a/data/*.json", "a/data/sub/y.json", false},
		{"other directory", "a/data/*.json", "a/other/y.json", false},
		{"rooted", "@data/*.json", "@data/y.json", true},
		{"question mark", "a/?.json", "a/y.json", true},
		{"question mark is one character", "a/?.json", "a/yy.json", false},
		{"class", "a/[xy].json", "a/y.json", true},
		{"braces", "a/*.{json,csv}", "a/y.csv", true},
		{"case sensitive", "a/*.json", "a/Y.JSON", false},
		{"dotfile needs a dot", "a/*.json", "a/.y.json", false},
		{"dotfile with a dot segment", "a/.*.json", "a/.y.json", true},
		{"double star, zero directories", "a/**/y.json", "a/y.json", true},
		{"double star, deep", "a/**/y.json", "a/b/c/y.json", true},
		{"double star skips dot directories", "a/**/y.json", "a/.b/y.json", false},
		{"trailing double star", "a/**", "a/b/c.json", true},
		{"trailing double star skips dot directories", "a/**", "a/.b/c.json", false},
		{"trailing double star skips dotfiles", "a/**", "a/.c.json", false},
		{"double star then star", "a/**/*.json", "a/b/y.json", true},
		{"directory segment", "a/*/y.json", "a/b/y.json", true},
		{"directory segment needs a directory", "a/*/y.json", "a/y.json", false},
		{"no glob names a file, which the single-file rule owns", "a/y.json", "a/y.json", false},
		{"directory only", "a/*/", "a/b", false},
		{"a backslash is refused by load", `a/*\x.json`, "a/y.json", false},
		{"an unclosed class is refused by load", "a/[x.json", "a/y.json", false},
		{"an empty segment is refused by load", "a//*.json", "a/b/y.json", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := load.MatchesPlanned(c.pattern, c.display); got != c.want {
				t.Fatalf("MatchesPlanned(%q, %q) = %v, want %v", c.pattern, c.display, got, c.want)
			}
		})
	}
}

// CODEGEN.md §2.4, DECISIONS 451: DirGlob splits a load.dir pattern with a glob as load's own split does.
func TestDirGlob(t *testing.T) {
	cases := []struct {
		name, call, base, rest string
		want                   bool
	}{
		{"glob", `load.dir("data/*.json")`, "data/", "*.json", true},
		{"rooted", `load.dir("@data/**/*.json")`, "@data/", "**/*.json", true},
		{"no base", `load.dir("*.json")`, "", "*.json", true},
		{"no glob", `load.dir("data/a.json")`, "", "", false},
		{"other method", `load.text("data/*.json")`, "", "", false},
		{"bare load", `load("data/*.json")`, "", "", false},
		{"backslash", `load.dir("data/*\\x.json")`, "", "", false},
		{"interpolated", `load.dir("data/{x}.json")`, "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base, rest, ok := load.DirGlob(loadCall(t, c.call))
			if base != c.base || rest != c.rest || ok != c.want {
				t.Fatalf("DirGlob(%s) = %q, %q, %v; want %q, %q, %v", c.call, base, rest, ok, c.base, c.rest, c.want)
			}
		})
	}
}
