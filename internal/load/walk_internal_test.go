package load

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

// fakeFS is a project.FS over an in-memory tree, names absolute and `/`-separated.
type fakeFS struct{ m fstest.MapFS }

func newFakeFS(files map[string]string) fakeFS {
	m := fstest.MapFS{}
	for name, data := range files {
		m[strings.TrimPrefix(name, "/")] = &fstest.MapFile{Data: []byte(data)}
	}
	return fakeFS{m}
}

func (f fakeFS) ReadFile(name string) ([]byte, error) {
	return f.m.ReadFile(strings.TrimPrefix(name, "/"))
}
func (f fakeFS) Stat(name string) (fs.FileInfo, error) {
	return f.m.Stat(strings.TrimPrefix(name, "/"))
}
func (f fakeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	n := strings.TrimPrefix(name, "/")
	if n == "" {
		n = "."
	}
	return f.m.ReadDir(n)
}

var _ project.FS = fakeFS{}

// testLoader is a Loader over an in-memory tree under "/p", with a Layout so walkBounds works.
func testLoader(t *testing.T, files map[string]string) *Loader {
	t.Helper()
	layout, ok := project.NewLayout(&project.Project{}, "/p", nil, diag.NewBag(nil, ""))
	if !ok {
		t.Fatal("layout")
	}
	return &Loader{FS: newFakeFS(files), Layout: layout}
}

// WIRE.md §6.5, §11: `**` descends every directory but a dotdir; matches sort in ascending byte order.
func TestGlobMatchesDoubleStarOrder(t *testing.T) {
	l := testLoader(t, map[string]string{
		"/p/data/b.json":             "{}",
		"/p/data/a.json":             "{}",
		"/p/data/.hidden.json":       "{}",
		"/p/data/sub/c.json":         "{}",
		"/p/data/sub/.git/keep.json": "{}",
	})
	matches, err := l.globMatches(project.Path{Display: "data", Abs: "/p/data"}, "**/*.json", Request{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range matches {
		got = append(got, m.Display)
	}
	want := []string{"data/a.json", "data/b.json", "data/sub/c.json"}
	if !slices.Equal(got, want) {
		t.Errorf("matches = %v, want %v", got, want)
	}
}

// meta/decisions/log-2026-09-24.md "load.dir review (M2)": consecutive "**" merge, and the
// result is deduped, so "**/**/*.json" walks and matches exactly like "**/*.json".
func TestGlobMatchesConsecutiveDoubleStar(t *testing.T) {
	l := testLoader(t, map[string]string{
		"/p/data/a.json":     "{}",
		"/p/data/sub/b.json": "{}",
	})
	matches, err := l.globMatches(project.Path{Display: "data", Abs: "/p/data"}, "**/**/*.json", Request{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range matches {
		got = append(got, m.Display)
	}
	want := []string{"data/a.json", "data/sub/b.json"}
	if !slices.Equal(got, want) {
		t.Errorf("matches = %v, want %v (no duplicates)", got, want)
	}
}

// WIRE.md §6.5: a trailing bare "**" matches every regular file at any depth under the base.
func TestGlobMatchesTrailingDoubleStar(t *testing.T) {
	l := testLoader(t, map[string]string{
		"/p/data/a.json":     "{}",
		"/p/data/sub/b.json": "{}",
		"/p/data/.hidden":    "{}",
	})
	matches, err := l.globMatches(project.Path{Display: "data", Abs: "/p/data"}, "**", Request{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range matches {
		got = append(got, m.Display)
	}
	want := []string{"data/a.json", "data/sub/b.json"}
	if !slices.Equal(got, want) {
		t.Errorf("matches = %v, want %v", got, want)
	}
}

// WIRE.md §6.5's dotfiles rule: a dotfile matches only a segment starting with a literal `.`.
func TestGlobMatchesDotSegment(t *testing.T) {
	l := testLoader(t, map[string]string{
		"/p/data/.config.json": "{}",
		"/p/data/plain.json":   "{}",
	})
	matches, err := l.globMatches(project.Path{Display: "data", Abs: "/p/data"}, ".*.json", Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Display != "data/.config.json" {
		t.Errorf("matches = %v, want just the dotfile", matches)
	}
}

// A glob with no magic character is the base itself; a directory matches no file; missing is E7004.
func TestGlobMatchesLiteral(t *testing.T) {
	l := testLoader(t, map[string]string{"/p/data/a.json": "{}"})
	matches, err := l.globMatches(project.Path{Display: "data/a.json", Abs: "/p/data/a.json"}, "", Request{})
	if err != nil || len(matches) != 1 || matches[0].Display != "data/a.json" {
		t.Errorf("existing file: matches = %v, err = %v", matches, err)
	}
	if matches, err := l.globMatches(project.Path{Display: "data", Abs: "/p/data"}, "", Request{}); err != nil || matches != nil {
		t.Errorf("an existing directory: matches = %v, err = %v", matches, err)
	}
	_, err = l.globMatches(project.Path{Display: "data/gone.json", Abs: "/p/data/gone.json"}, "", Request{})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing literal path: err = %v, want fs.ErrNotExist", err)
	}
}

// meta/decisions/log-2026-09-24.md "load.dir review (M2)": a missing glob base is E7004, not a
// silent empty match; only what the glob itself walks past the base may be legitimately absent.
func TestGlobMatchesMissingBase(t *testing.T) {
	l := testLoader(t, map[string]string{"/p/other/x.json": "{}"})
	_, err := l.globMatches(project.Path{Display: "data", Abs: "/p/data"}, "*.json", Request{})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want fs.ErrNotExist", err)
	}
}

// meta/decisions/log-2026-09-24.md "load.dir review (M2)": a glob base naming a file, not a
// directory, is E7004's "not a directory" cause.
func TestGlobMatchesBaseIsFile(t *testing.T) {
	l := testLoader(t, map[string]string{"/p/data.json": "{}"})
	_, err := l.globMatches(project.Path{Display: "data.json", Abs: "/p/data.json"}, "*.json", Request{})
	if !errors.Is(err, errNotDir) {
		t.Errorf("err = %v, want errNotDir", err)
	}
}

// WIRE.md §6.5: a directory that does not exist yet is zero matches, not an error.
func TestReadDirMissing(t *testing.T) {
	l := &Loader{FS: newFakeFS(nil)}
	entries, err := l.readDir("/p/gone")
	if err != nil || entries != nil {
		t.Errorf("readDir of a missing directory = %v, %v", entries, err)
	}
}

// WIRE.md §6.5 (R2-4): resolved '/'-separated paths, compared by whole segments.
func TestWithin(t *testing.T) {
	cases := []struct {
		real   string
		bounds []string
		want   bool
	}{
		{"/p/data/a.json", []string{"/p"}, true},
		{"/p", []string{"/p"}, true},
		{"/px/a.json", []string{"/p"}, false},
		{"/q/a.json", []string{"/p", "/q"}, true},
		{"C:/proj/data/a.json", []string{"C:/proj"}, true},
		{"C:/project/a.json", []string{"C:/proj"}, false},
		{"C:/x/a.json", []string{"C:/"}, true},
		{"/anything", []string{"/"}, true},
		{`C:\proj\a.json`, []string{"C:/proj"}, false},
	}
	for _, c := range cases {
		if got := within(c.real, c.bounds); got != c.want {
			t.Errorf("within(%q, %v) = %v, want %v", c.real, c.bounds, got, c.want)
		}
	}
}

// WIRE.md §2.3 (R2-3): one resolved file matched twice keeps its first match in path order.
func TestFirstByReal(t *testing.T) {
	hits := []hit{
		{rel: "real/b.json", real: "/p/data/real/b.json"},
		{rel: "a.json", real: "/p/data/a.json"},
		{rel: "gooddir/b.json", real: "/p/data/real/b.json"},
		{rel: "sub/a.json", real: "/p/data/sub/a.json"},
	}
	var got []string
	for _, h := range firstByReal(hits) {
		got = append(got, h.rel)
	}
	if want := []string{"a.json", "gooddir/b.json", "sub/a.json"}; !slices.Equal(got, want) {
		t.Errorf("firstByReal = %v, want %v", got, want)
	}
}
