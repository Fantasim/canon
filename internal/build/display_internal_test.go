package build

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
)

// The separators relativeTo is tested with: a Windows path's, built explicitly rather than from
// GOOS, and a POSIX one's.
const (
	winSep   = '\\'
	posixSep = '/'
	projDir  = "/x/proj"
)

// DECISIONS 201, IMPLEMENTATION-PLAN.md §10: an OS path prints '/'-separated, never absolute.
func TestRelativeTo(t *testing.T) {
	for _, c := range []struct {
		name, dir, path string
		sep             rune
		want            string
	}{
		{"windows nested", "C:/proj", `C:\proj\a\a.canon`, winSep, "a/a.canon"},
		{"windows project dir", "C:/proj", `C:\proj`, winSep, listingMark},
		{"windows outside", "C:/proj", `D:\other\x.canon`, winSep, "x.canon"},
		{"windows drive root project", "C:/", `C:\a.canon`, winSep, "a.canon"},
		{"nested", projDir, projDir + "/a/b.canon", posixSep, "a/b.canon"},
		{"project dir", projDir, projDir, posixSep, listingMark},
		{"project dir, trailing separator", projDir + "/", projDir, posixSep, listingMark},
		{"sibling sharing a prefix", projDir, "/x/proj2/a", posixSep, "a"},
		{"root project", "/", "/a.canon", posixSep, "a.canon"},
		{"file system root outside", projDir, "/", posixSep, listingMark},
		{"empty", projDir, "", posixSep, listingMark},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := relativeTo(c.dir, c.path, c.sep); got != c.want {
				t.Errorf("relativeTo(%q, %q) = %q, want %q", c.dir, c.path, got, c.want)
			}
		})
	}
}

// DECISIONS 201: a *fs.PathError or *os.LinkError is named by its display path, cause kept.
func TestDisplayErrorIn(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want string
	}{
		{"nested", &fs.PathError{Op: "open", Path: projDir + "/a/a.canon", Err: fs.ErrPermission}, "a/a.canon: open: permission denied"},
		{"project dir", &fs.PathError{Op: "open", Path: projDir, Err: fs.ErrPermission}, ".: open: permission denied"},
		{"sibling", &fs.PathError{Op: "open", Path: "/x/proj2/a", Err: fs.ErrPermission}, "a: open: permission denied"},
		{"wrapped", fmt.Errorf("%w", &fs.PathError{Op: "read", Path: projDir + "/p.canon", Err: fs.ErrPermission}), "p.canon: read: permission denied"},
		{"link error", &os.LinkError{Op: "rename", Old: projDir + "/o/.v.json.canon-tmp", New: projDir + "/o/v.json", Err: fs.ErrPermission}, "o/v.json: rename: permission denied"},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := displayErrorIn(projDir, c.err)
			if msg := err.Error(); msg != c.want || strings.Contains(msg, "/x/") {
				t.Errorf("displayErrorIn = %q, want %q", msg, c.want)
			}
			if !errors.Is(err, fs.ErrPermission) {
				t.Errorf("displayErrorIn(%v) lost fs.ErrPermission", c.err)
			}
		})
	}
	if err := displayErrorIn(projDir, errListing); err != errListing {
		t.Errorf("an error with no path = %v, want it unchanged", err)
	}
}
