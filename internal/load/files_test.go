package load_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// callOf parses one load call written as a `let` in a file of the project and returns its expression.
func callOf(t *testing.T, text string) *syntax.LoadExpr {
	t.Helper()
	set := &source.FileSet{}
	src, err := set.Add("a/a.canon", "/a/a.canon", []byte("package a\n\nlet x: Int = "+text+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	var out *syntax.LoadExpr
	syntax.Inspect(syntax.Parse(src, syntax.FileSource, diag.NewBag(set, "")), func(n syntax.Node) bool {
		if e, ok := n.(*syntax.LoadExpr); ok {
			out = e
		}
		return true
	})
	if out == nil {
		t.Fatalf("no load in %q", text)
	}
	return out
}

// WIRE.md §6.1, §6.2, §6.5: JSONFiles is the files a call reads as JSON, by the loader's own rules.
func TestJSONFiles(t *testing.T) {
	skipOnWindows(t)
	checkJSONFiles(t, filepath.ToSlash(t.TempDir()))
}

// WIRE.md §6.5, "load.dir round 3": a project under a linked parent (macOS's /var) reads as without it.
func TestJSONFilesUnderLinkedParent(t *testing.T) {
	skipOnWindows(t)
	top := t.TempDir()
	if err := os.Mkdir(filepath.Join(top, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(top, "real"), filepath.Join(top, "alias")); err != nil {
		t.Fatal(err)
	}
	checkJSONFiles(t, filepath.ToSlash(filepath.Join(top, "alias")))
}

// checkJSONFiles checks JSONFiles and Inside on a project and an outside directory made in root;
// a resolved path is root's own, its links resolved.
func checkJSONFiles(t *testing.T, root string) {
	t.Helper()
	mkTree(t, root, map[string]string{
		"proj/d/a1.json": "{}", "proj/d/b1.json": "{}", "proj/d/sub/c.json": "{}", "proj/d/t.txt": "{}", "proj/d/.h.json": "{}",
		"proj/real/x.json": "{}", "outside/o.json": "{}",
	}, map[string]string{"proj/d/in.json": "proj/real/x.json", "proj/d/out.json": "outside/o.json"})
	proj := root + "/proj"
	layout := layoutAt(t, proj)
	tests := []struct {
		name, call string
		want       []string
	}{
		{"plain", `load("../d/a1.json")`, []string{"d/a1.json"}},
		{"format text wins", `load("../d/a1.json", format: text)`, nil},
		{"format json wins", `load("../d/t.txt", format: json)`, []string{"d/t.txt"}},
		{"extension decides", `load("../d/t.txt")`, nil},
		{"negated class", `load.dir("../d/[!a]*.json")`, []string{"d/b1.json", "d/in.json"}},
		{"caret is a member, not a negation", `load.dir("../d/[^a]*.json")`, []string{"d/a1.json"}},
		{"deep glob", `load.dir("../d/**/*.json")`, []string{"d/a1.json", "d/b1.json", "d/in.json", "d/sub/c.json"}},
		{"dotfile", `load.dir("../d/.*.json")`, []string{"d/.h.json"}},
		{"double star not a segment", `load.dir("../d/**x.json")`, nil},
		{"unclosed class", `load.dir("../d/[a.json")`, nil},
		{"backslash", `load.dir("../d/\\a1.json")`, nil},
		{"other format", `load.dir("../d/*.json", format: csv)`, nil},
		{"mixed extensions", `load.dir("../d/*")`, nil},
		{"forced", `load.dir("../d/*", format: json)`, []string{"d/a1.json", "d/b1.json", "d/in.json", "d/t.txt"}},
		{"link inside", `load("../d/in.json")`, []string{"d/in.json"}},
		{"link outside", `load("../d/out.json")`, nil},
		{"literal dir path through a link outside", `load.dir("../d/out.json")`, nil},
		{"not literal", `load(P)`, nil},
		{"other form", `load.text("../d/a1.json")`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, f := range load.JSONFiles(project.OS(), layout, "a", callOf(t, tt.call), diag.NewBag(nil, "")) {
				got = append(got, f.Display)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
	if got := load.JSONFiles(brokenLinks{project.OS()}, layout, "a", callOf(t, `load("../d/a1.json")`), diag.NewBag(nil, "")); len(got) != 0 {
		t.Errorf("link resolution fails, yet JSONFiles read %v", got)
	}
	resolved, err := filepath.EvalSymlinks(filepath.FromSlash(proj))
	if err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]string{"/d/in.json": "/real/x.json", "/d/a1.json": "/d/a1.json"} {
		want = filepath.ToSlash(resolved) + want
		if real, ok := load.Inside(project.OS(), layout, proj+file); !ok || real != want {
			t.Errorf("Inside %s in the project: %q, %v; want %q", file, real, ok, want)
		}
	}
	if _, ok := load.Inside(project.OS(), layout, proj+"/d/out.json"); ok {
		t.Error("Inside a link out of the project")
	}
	if _, ok := load.Inside(project.OS(), layout, proj+"/d/missing.json"); ok {
		t.Error("Inside a missing file")
	}
}

// brokenLinks is a file system whose link resolution fails, neither for a missing file nor for a loop.
type brokenLinks struct{ project.FS }

var errBroken = errors.New("resolution failed")

func (brokenLinks) EvalSymlinks(string) (string, error) { return "", errBroken }

// WIRE.md §6.5: a link that does not resolve is never read as inside the roots, so fmt never replaces it.
func TestInsideLinkResolutionFails(t *testing.T) {
	root := filepath.ToSlash(t.TempDir())
	mkTree(t, root, map[string]string{"proj/d/a.json": "{}"}, nil)
	if real, ok := load.Inside(brokenLinks{project.OS()}, layoutAt(t, root+"/proj"), root+"/proj/d/a.json"); ok || real != "" {
		t.Errorf("Inside: %q, %v", real, ok)
	}
}

// TestInsideWithoutLinks: without links every file is its own real path (WIRE.md §6.5).
func TestInsideWithoutLinks(t *testing.T) {
	root := t.TempDir()
	abs := root + "/proj/d/a.json"
	if real, ok := load.Inside(noLinks{project.OS()}, layoutAt(t, root+"/proj"), abs); !ok || real != abs {
		t.Fatalf("Inside = %q, %v; want %q, true", real, ok, abs)
	}
}
