package load_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
)

// WIRE.md §6.5: Files is every file a call reads whatever its format; JSONFiles its JSON part.
func TestFiles(t *testing.T) {
	skipOnWindows(t)
	root := filepath.ToSlash(t.TempDir())
	mkTree(t, root, map[string]string{
		"proj/d/a1.json": "{}", "proj/d/r.csv": "a\n1\n", "proj/d/t.txt": "x", "proj/d/sub/c.csv": "a\n",
		"outside/o.csv": "a\n",
	}, map[string]string{"proj/d/out.csv": "outside/o.csv"})
	layout := layoutAt(t, root+"/proj")
	tests := []struct {
		name, call string
		want       []string
	}{
		{"csv glob", `load.dir("../d/*.csv")`, []string{"d/r.csv"}},
		{"deep csv glob", `load.dir("../d/**/*.csv")`, []string{"d/r.csv", "d/sub/c.csv"}},
		{"text glob", `load.dir("../d/*.txt")`, []string{"d/t.txt"}},
		{"every format", `load.dir("../d/*")`, []string{"d/a1.json", "d/r.csv", "d/t.txt"}},
		{"format option", `load.dir("../d/*.json", format: csv)`, []string{"d/a1.json"}},
		{"csv with its own option", `load.dir("../d/*.csv", partial: true)`, []string{"d/r.csv"}},
		{"csv with an option csv refuses", `load.dir("../d/*.csv", at: "a")`, nil},
		{"text with an option text refuses", `load.dir("../d/*.txt", partial: true)`, nil},
		{"format option decides the options", `load.dir("../d/*.json", format: text, partial: true)`, nil},
		{"plain csv", `load("../d/r.csv")`, []string{"d/r.csv"}},
		{"plain text wins", `load("../d/a1.json", format: text)`, []string{"d/a1.json"}},
		{"link outside", `load("../d/out.csv")`, nil},
		{"unclosed class", `load.dir("../d/[a.csv")`, nil},
		{"not literal", `load(P)`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, f := range load.Files(project.OS(), layout, "a", callOf(t, tt.call), diag.NewBag(nil, "")) {
				got = append(got, f.Display)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
