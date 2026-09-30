package project

import "testing"

// winPaths is how Windows writes names, its volumes read as text (volumeOf).
var winPaths = Paths{Sep: backslash, Volume: volumeOf}

// API.md §2.2 (log M4 B12-r): on Windows a drive or UNC volume is kept, a drive upper-cased.
func TestPathsOnWindows(t *testing.T) {
	ops := map[string]func(string) string{
		"clean": winPaths.clean,
		"dir":   winPaths.dir,
		"api":   func(name string) string { return winPaths.FromAPI(name, "d:/law") },
	}
	cases := []struct{ op, in, want string }{
		{"clean", "C:/..", "C:/"},
		{"clean", "c:/a/../b", "C:/b"},
		{"clean", "C:", "C:/"},
		{"clean", "//server/share/x/../y", "//server/share/y"},
		{"clean", "//server/share", "//server/share/"},
		{"clean", "/a/../b", "/b"},
		{"clean", "a/./b", "a/b"},
		{"dir", "//server/share/proj", "//server/share/"},
		{"dir", "//server/share/", "//server/share/"},
		{"dir", "C:/law", "C:/"},
		{"dir", "C:/", "C:/"},
		{"api", `\\server\share\proj\a.canon`, "//server/share/proj/a.canon"},
		{"api", `\\wsl$\Ubuntu\home\a.canon`, "//wsl$/Ubuntu/home/a.canon"},
		{"api", `\law\a.canon`, "D:/law/a.canon"},
		{"api", "d:/law/a.canon", "D:/law/a.canon"},
		{"api", `a\b.canon`, "a/b.canon"},
		{"api", "/..", "D:/"},
	}
	for _, c := range cases {
		if got := ops[c.op](c.in); got != c.want {
			t.Errorf("%s(%q) = %q, want %q", c.op, c.in, got, c.want)
		}
	}
}

// API.md §2.2 (log M4 B12-r): on Windows a join keeps its volume and never climbs above it.
func TestJoinOnWindows(t *testing.T) {
	cases := []struct {
		dir  string
		elem []string
		want string
	}{
		{"//server/share/proj", []string{"a", "b.canon"}, "//server/share/proj/a/b.canon"},
		{"//wsl$/Ubuntu/home", []string{"..", "..", "a"}, "//wsl$/Ubuntu/a"},
		{"C:/", []string{"a"}, "C:/a"},
		{"d:/law", []string{"../../r"}, "D:/r"},
		{"law", []string{"a"}, "law/a"},
	}
	for _, c := range cases {
		if got := winPaths.Join(c.dir, c.elem...); got != c.want {
			t.Errorf("Join(%q, %q) = %q, want %q", c.dir, c.elem, got, c.want)
		}
	}
}

// API.md §2.2: on a system without volumes the host's Paths is package path's.
func TestHostPathsWithoutVolumes(t *testing.T) {
	if HostPaths().Volume("C:/x") != "" {
		t.Skip("the host has volumes")
	}
	if got := Join("/law", "..", "..", "x"); got != "/x" {
		t.Errorf("Join = %q", got)
	}
	if got := DirOf("/"); got != "/" {
		t.Errorf("DirOf = %q", got)
	}
	if got := Clean("/law/./a"); got != "/law/a" {
		t.Errorf("Clean = %q", got)
	}
}
