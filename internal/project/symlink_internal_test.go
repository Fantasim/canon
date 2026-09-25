package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// WIRE.md §2.1, §6.5: volumeOf is parsed as text, so it is the same on every GOOS.
func TestVolumeOf(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/p/data", ""},
		{"C:/p/data", "C:"},
		{"C:", "C:"},
		{"c:/x", "c:"},
		{"//server/share/x", "//server/share"},
		{"//server/share", "//server/share"},
		{"//server", ""},
		{"//server/", ""},
		{"relative/x", ""},
	}
	for _, c := range cases {
		if got := volumeOf(c.in); got != c.want {
			t.Errorf("volumeOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// WIRE.md §6.5: ".." never climbs above a path's own volume, drive letter or UNC share included.
func TestApplyDotSegment(t *testing.T) {
	cases := []struct {
		dest, seg, want string
		ok              bool
	}{
		{"/a", "..", "", true},
		{"", "..", "", true},
		{"/a/b", "..", "/a", true},
		{"C:", "..", "C:", true},
		{"C:/a", "..", "C:", true},
		{"//server/share", "..", "//server/share", true},
		{"//server/share/a", "..", "//server/share", true},
		{"/a", ".", "/a", true},
		{"/a", "", "/a", true},
		{"/a", "b", "", false},
	}
	for _, c := range cases {
		got, ok := applyDotSegment(c.dest, c.seg)
		if got != c.want || ok != c.ok {
			t.Errorf("applyDotSegment(%q, %q) = %q, %v; want %q, %v", c.dest, c.seg, got, ok, c.want, c.ok)
		}
	}
}

// WIRE.md §6.5: a link naming its own volume restarts there, one without stays on the current volume when absolute or under dest when relative.
func TestFollowLink(t *testing.T) {
	cases := []struct {
		dest, link  string
		pending     []string
		wantDest    string
		wantPending []string
	}{
		{"/a", "b", []string{"c"}, "/a", []string{"b", "c"}},
		{"/a", "/x/y", []string{"c"}, "", []string{"x", "y", "c"}},
		{"C:/a", "/x", nil, "C:", []string{"x"}},
		{"C:/a", "D:/x", nil, "D:", []string{"x"}},
		{"//server/share/a", "/x", nil, "//server/share", []string{"x"}},
	}
	for _, c := range cases {
		dest, pending := followLink(c.dest, c.link, c.pending)
		if dest != c.wantDest || !equalSegs(pending, c.wantPending) {
			t.Errorf("followLink(%q, %q, %v) = %q, %v; want %q, %v", c.dest, c.link, c.pending, dest, pending, c.wantDest, c.wantPending)
		}
	}
}

func equalSegs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// symlinkTree builds relative, absolute, chained, trailing, self and mutual links under a fresh
// temp dir (already EvalSymlinks'd, so the real disk's own aliasing does not confuse a case).
func symlinkTree(t *testing.T) string {
	t.Helper()
	skipOnWindows(t)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, "target.txt", "")
	write(t, dir, "sub/inside.txt", "")
	symlink(t, "target.txt", filepath.Join(dir, "rel_link"))
	symlink(t, filepath.Join(dir, "target.txt"), filepath.Join(dir, "abs_link"))
	symlink(t, "abs_link", filepath.Join(dir, "chain_a"))
	symlink(t, filepath.Join(dir, "sub"), filepath.Join(dir, "dir_link"))
	symlink(t, "../target.txt", filepath.Join(dir, "sub", "trailing_link"))
	symlink(t, filepath.Join(dir, "self_loop"), filepath.Join(dir, "self_loop"))
	symlink(t, filepath.Join(dir, "mutual_b"), filepath.Join(dir, "mutual_a"))
	symlink(t, filepath.Join(dir, "mutual_a"), filepath.Join(dir, "mutual_b"))
	symlink(t, filepath.Join(dir, "gone"), filepath.Join(dir, "dangling"))
	return dir
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevated privilege on windows")
	}
}

// WIRE.md §6.5: evalSymlinksOS resolves relative, absolute, chained and trailing links, applies ".." after a link, and reports a self or mutual loop as ErrSymlinkLoop, never OS text.
func TestEvalSymlinksOS(t *testing.T) {
	dir := symlinkTree(t)
	cases := []struct {
		name, path, want string
		wantLoop         bool
		wantNotExist     bool
	}{
		{"relative", "rel_link", "target.txt", false, false},
		{"absolute", "abs_link", "target.txt", false, false},
		{"chained", "chain_a", "target.txt", false, false},
		{"trailing", "sub/trailing_link", "target.txt", false, false},
		{"dotdot after a link", "dir_link/../target.txt", "target.txt", false, false},
		{"self loop", "self_loop", "", true, false},
		{"mutual loop", "mutual_a", "", true, false},
		{"enoent", "dangling", "", false, true},
		{"enoent component", "nosuch/x", "", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := evalSymlinksOS(dir + "/" + c.path)
			switch {
			case c.wantLoop:
				if !errors.Is(err, ErrSymlinkLoop) {
					t.Errorf("err = %v, want ErrSymlinkLoop", err)
				}
			case c.wantNotExist:
				if !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("err = %v, want fs.ErrNotExist", err)
				}
			default:
				if want := dir + "/" + c.want; err != nil || got != want {
					t.Errorf("evalSymlinksOS(%q) = %q, %v; want %q, nil", c.path, got, err, want)
				}
			}
		})
	}
}

// WIRE.md §6.5: a non-directory path component is its own FS error, ENOTDIR on Unix; project reports no Kind of its own for it.
func TestEvalSymlinksOSNotDir(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	write(t, dir, "plain.txt", "")
	if _, err := evalSymlinksOS(dir + "/plain.txt/x"); err == nil {
		t.Error("err = nil, want a non-directory error")
	}
}
