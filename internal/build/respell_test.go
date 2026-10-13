package build_test

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

const respellRule = "DECISIONS 357, CODEGEN.md §2.9: "

// caseFS is a case-insensitive file system that keeps each name's spelling, as macOS and Windows
// do: over a mapFS whose keys are the spellings on disk, a name finds the key equal to it but for
// case, a new file takes the spelling it is created with and a rename the new one.
type caseFS struct {
	mapFS
}

// find is the spelling on disk of name, a file or a directory holding one; name itself for none.
func (f caseFS) find(name string) string {
	want := strings.TrimPrefix(name, "/")
	for _, k := range slices.Sorted(maps.Keys(f.mapFS)) {
		switch {
		case strings.EqualFold(k, want):
			return "/" + k
		case len(k) > len(want) && strings.EqualFold(k[:len(want)+1], want+"/"):
			return "/" + k[:len(want)]
		}
	}
	return name
}

func (f caseFS) ReadFile(name string) ([]byte, error)  { return f.mapFS.ReadFile(f.find(name)) }
func (f caseFS) Stat(name string) (fs.FileInfo, error) { return f.mapFS.Stat(f.find(name)) }
func (f caseFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return f.mapFS.ReadDir(f.find(name))
}
func (f caseFS) WriteFile(name string, data []byte) error {
	return f.mapFS.WriteFile(f.find(name), data)
}
func (f caseFS) Remove(name string) error { return f.mapFS.Remove(f.find(name)) }
func (f caseFS) Rename(oldname, newname string) error {
	from := f.find(oldname)
	data, ok := f.mapFS[strings.TrimPrefix(from, "/")]
	if !ok {
		return fs.ErrNotExist
	}
	delete(f.mapFS, strings.TrimPrefix(from, "/"))
	delete(f.mapFS, strings.TrimPrefix(f.find(newname), "/"))
	f.mapFS[strings.TrimPrefix(newname, "/")] = data
	return nil
}

// textFn is package a writing one @text file of the given name and content into @out/sql.
func textFn(name, content string) string {
	return "/// P.\npackage a\n\n/// F.\n@text(\"" + name + "\")\nexport fn f() -> String { return \"" + content + "\" }\n\nemit text { out: \"@out/sql\" }\n"
}

// sqlNames are the file names of p/out/sql in fsys, sorted.
func sqlNames(fsys mapFS) []string {
	var out []string
	for name := range fsys { //canon:unordered sorted below
		if rest, ok := strings.CutPrefix(name, "p/out/sql/"); ok {
			out = append(out, rest)
		}
	}
	slices.Sort(out)
	return out
}

// DECISIONS 357: on a case-insensitive file system a case-only rename of a text file is no E8001:
// the old line of canon.outputs owns the new spelling, which lands, new content or not; --check
// reports it stale first.
func TestCaseOnlyTextRename(t *testing.T) {
	for _, c := range []struct {
		name, content string
	}{
		{"changed content", "B"},
		{"unchanged content", "A"},
	} {
		fsys := caseFS{mapFS{"p/project.canon": file(textProject), "p/a/a.canon": file(textFn("S.sql", "A"))}}
		buildOn(t, fsys, build.BuildOptions{})
		fsys.mapFS["p/a/a.canon"] = file(textFn("s.sql", c.content))
		check := buildOn(t, fsys, build.BuildOptions{Check: true})
		if check.Summary.Errors != 0 || !check.Stale || statusOf(t, check, "@out/sql/s.sql") != build.StatusStale {
			t.Errorf("%s%s: --check %v, stale %v", respellRule, c.name, codes(check.List), check.Stale)
		}
		res := buildOn(t, fsys, build.BuildOptions{})
		names := sqlNames(fsys.mapFS)
		if res.Summary.Errors != 0 || !slices.Equal(names, []string{"s.sql"}) || string(fsys.mapFS["p/out/sql/s.sql"].Data) != c.content {
			t.Errorf("%s%s: %v, files %v", respellRule, c.name, codes(res.List), names)
		}
		if listing := string(fsys.mapFS["p/a/canon.outputs"].Data); !strings.Contains(listing, "@out/sql/s.sql") || strings.Contains(listing, "S.sql") {
			t.Errorf("%s%s: canon.outputs %q", respellRule, c.name, listing)
		}
		if again := buildOn(t, fsys, build.BuildOptions{Check: true}); again.Stale || again.Summary.Errors != 0 {
			t.Errorf("%s%s: not fresh after the build: %v", respellRule, c.name, codes(again.List))
		}
	}
}

// DECISIONS 357: an unlisted file differing from the declared name in case alone stays canon's
// to refuse: E8001 when its bytes differ, and no rename.
func TestCaseOnlyUnlistedRefused(t *testing.T) {
	fsys := caseFS{mapFS{"p/project.canon": file(textProject), "p/a/a.canon": file(textFn("s.sql", "B")), "p/out/sql/S.sql": file("hand")}}
	res := buildOn(t, fsys, build.BuildOptions{})
	if !slices.Contains(codes(res.List), diag.E8001.Def().Code) || !slices.Equal(sqlNames(fsys.mapFS), []string{"S.sql"}) {
		t.Errorf("%s%v, files %v", respellRule, codes(res.List), sqlNames(fsys.mapFS))
	}
}

// failingCaseFS is a caseFS whose renames fail when fails says so.
type failingCaseFS struct {
	caseFS
	fails func(oldname, newname string) bool
}

func (f failingCaseFS) Rename(oldname, newname string) error {
	if f.fails(oldname, newname) {
		return errInjected
	}
	return f.caseFS.Rename(oldname, newname)
}

// DECISIONS 357, API.md N11: a respelled file whose build fails later on, or whose new content
// fails to land, is put back under its old spelling with its old bytes, nothing staged left.
func TestCaseOnlyRenameRollback(t *testing.T) {
	for _, c := range []struct {
		name  string
		fails func(oldname, newname string) bool
	}{
		{"a later change fails", func(_, n string) bool { return strings.HasSuffix(n, "/canon.outputs") }},
		{"the new content fails", func(o, n string) bool { return strings.HasSuffix(n, "/s.sql") && strings.Contains(o, "/.s.sql.") }},
	} {
		base := caseFS{mapFS{"p/project.canon": file(textProject), "p/a/a.canon": file(textFn("S.sql", "A"))}}
		buildOn(t, base, build.BuildOptions{})
		base.mapFS["p/a/a.canon"] = file(textFn("s.sql", "B"))
		p, err := build.Open(failingCaseFS{caseFS: base, fails: c.fails}, "/p", build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.Build(context.Background(), build.BuildOptions{}); !errors.Is(err, errInjected) {
			t.Fatalf("%s%s: %v", respellRule, c.name, err)
		}
		if names := sqlNames(base.mapFS); !slices.Equal(names, []string{"S.sql"}) || string(base.mapFS["p/out/sql/S.sql"].Data) != "A" {
			t.Errorf("%s%s: files %v", respellRule, c.name, names)
		}
	}
}
