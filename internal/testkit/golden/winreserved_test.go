package golden

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const (
	repoRootDir = "../../.."
	// skippedDirs are not checked out by Windows CI: git's own, and the git-ignored real data.
	gitDir      = ".git"
	realDataDir = "testdata-real"
	comPrefix   = "com"
	lptPrefix   = "lpt"
	comLptSpan  = 10
)

// reservedDevices are the Windows reserved device names, without COM0-9 and LPT0-9 (reservedName adds them).
var reservedDevices = map[string]bool{"con": true, "prn": true, "aux": true, "nul": true}

// reservedName reports whether base, up to its first dot and in any letter case, is a Windows reserved device name.
func reservedName(base string) bool {
	stem, _, _ := strings.Cut(strings.ToLower(base), ".")
	if reservedDevices[stem] {
		return true
	}
	for _, prefix := range []string{comPrefix, lptPrefix} {
		for d := range comLptSpan {
			if stem == prefix+string(rune('0'+d)) {
				return true
			}
		}
	}
	return false
}

// Windows reserved names: no path in the repository may have a component git cannot check out on Windows.
func TestNoWindowsReservedPaths(t *testing.T) {
	err := filepath.WalkDir(repoRootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == gitDir || d.Name() == realDataDir) {
			return filepath.SkipDir
		}
		if reservedName(d.Name()) {
			t.Errorf("%s: a Windows reserved device name", filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Windows reserved names: the match is case-insensitive, ignores every extension and spares look-alikes.
func TestReservedName(t *testing.T) {
	cases := map[string]bool{
		"nul.gen.cpp": true, "NUL": true, "Con.txt": true, "aux": true, "prn.h": true, "com1.x": true, "LPT9": true,
		"nulstr.gen.cpp": false, "com10": false, "lpt": false, "console": false, "a.nul": false,
	}
	for name, want := range cases {
		if got := reservedName(name); got != want {
			t.Errorf("reservedName(%q) = %v, want %v", name, got, want)
		}
	}
}
