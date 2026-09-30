package project

import (
	"maps"
	"testing"
)

// API.md §2.2: Volume reads filepath.VolumeName's backslash output as a '/' name, and only a drive or a UNC share is a volume.
func TestNewPathsVolumes(t *testing.T) {
	cases := []struct{ name, want string }{
		{"C:/x", "C:"},
		{`c:\x`, "c:"},
		{"//server/share/x", "//server/share"},
		{`\\server\share\x`, "//server/share"},
		{"//wsl$/Ubuntu/home", "//wsl$/Ubuntu"},
		{"/law/x", ""},
		{"law/x", ""},
		{"//server", ""},
		{"//server/", ""},
		{"//?/C:/x", ""},
		{`\\?\UNC\host\share\x`, ""},
		{"//./UNC/host/share/x", ""},
		{`\\.\COM1`, ""},
	}
	for _, c := range cases {
		if got := winPaths.Volume(c.name); got != c.want {
			t.Errorf("Volume(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// API.md §2.2: device paths and a UNC host with no share are not supported: rooted names on the directory's volume, the same on every host.
func TestUnsupportedVolumes(t *testing.T) {
	cases := []struct{ name, want string }{
		{"//?/C:/x", "D:/?/C:/x"},
		{`\\.\COM1`, "D:/COM1"},
		{"//server", "D:/server"},
		{"//server/", "D:/server"},
		{"//server/share", "//server/share"},
	}
	for _, c := range cases {
		if got := winPaths.FromAPI(c.name, "d:/law"); got != c.want {
			t.Errorf("FromAPI(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// API.md §2.1, §2.2: roots given to the API become FS names on the project's volume, relative ones stay relative, nil stays nil.
func TestRootsFromAPI(t *testing.T) {
	if winPaths.RootsFromAPI(nil, "D:/law") != nil {
		t.Error("no roots became some")
	}
	in := map[string]string{"unc": `\\server\share\out`, "rooted": `\x\..\y`, "rel": `gen\ts`, "drive": `c:\gen`}
	want := map[string]string{"unc": "//server/share/out", "rooted": "D:/y", "rel": "gen/ts", "drive": "C:/gen"}
	got := winPaths.RootsFromAPI(in, "d:/law")
	if !maps.Equal(got, want) {
		t.Errorf("roots %v, want %v", got, want)
	}
	got["rel"] = "changed"
	if in["rel"] != `gen\ts` {
		t.Error("the caller's roots were changed")
	}
}
