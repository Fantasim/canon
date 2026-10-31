package project

import (
	"errors"
	"io/fs"
	"strings"
	"syscall"
	"testing"
)

// fakeEntry is one name of fakeLinks: its Lstat mode and what readlink gives, its target or,
// when set, err; an entry with neither fails with EINVAL, as Go's readlink of a non-link does.
type fakeEntry struct {
	mode   fs.FileMode
	target string
	err    error
}

// fakeLinks is a linkReader over '/' names, a Windows disk modelled on Linux: a junction is
// ModeIrregular with a target, a placeholder ModeIrregular without one.
type fakeLinks map[string]fakeEntry

func (f fakeLinks) lstat(name string) (fs.FileMode, error) {
	e, ok := f[name]
	if !ok {
		return 0, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrNotExist}
	}
	return e.mode, nil
}

func (f fakeLinks) readlink(name string) (string, error) {
	e, ok := f[name]
	switch {
	case ok && e.err != nil:
		return "", &fs.PathError{Op: "readlink", Path: name, Err: e.err}
	case !ok || e.target == "":
		return "", &fs.PathError{Op: "readlink", Path: name, Err: syscall.EINVAL}
	}
	return e.target, nil
}

// windowsDisk holds junctions, a volume and a device mount point, a placeholder and symlinks.
var windowsDisk = fakeLinks{
	"C:/p":         {mode: fs.ModeDir},
	"C:/q":         {mode: fs.ModeDir},
	"C:/q/x":       {},
	"C:/p/j":       {mode: fs.ModeIrregular, target: "C:/q"},
	"C:/p/k":       {mode: fs.ModeIrregular, target: "C:/p/j"},
	"C:/p/r":       {mode: fs.ModeIrregular, target: "D:/"},
	"C:/p/l1":      {mode: fs.ModeIrregular, target: "C:/p/l2"},
	"C:/p/l2":      {mode: fs.ModeIrregular, target: "C:/p/l1"},
	"C:/p/v":       {mode: fs.ModeIrregular, target: "//?/Volume{0b1c}/"},
	"C:/p/d":       {mode: fs.ModeIrregular, target: "//./PhysicalDrive0"},
	"C:/p/h":       {mode: fs.ModeIrregular},
	"C:/p/s":       {mode: fs.ModeSymlink, target: "j"},
	"C:/p/sd":      {mode: fs.ModeSymlink, target: "//?/C:/q"},
	"C:/p/mixed":   {mode: fs.ModeSymlink, target: "k/x"},
	"C:/p/through": {mode: fs.ModeIrregular, target: "C:/p/s"},
	"C:/p/gone":    {mode: fs.ModeIrregular, err: fs.ErrNotExist},
	"C:/p/denied":  {mode: fs.ModeIrregular, err: fs.ErrPermission},
	"C:/p/broken":  {mode: fs.ModeSymlink, err: syscall.EINVAL},
}

// WIRE.md §6.5, DECISIONS 419: a junction is followed, chains included; a loop is ErrSymlinkLoop; a device target is fs.ErrNotExist naming no path; an irregular non-link resolves to itself; symlinks are unchanged.
func TestEvalSymlinksJunction(t *testing.T) {
	cases := []struct {
		name, in, want string
		wantErr        error
	}{
		{"junction", "C:/p/j/x", "C:/q/x", nil},
		{"junction itself", "C:/p/j", "C:/q", nil},
		{"junction chain", "C:/p/k/x", "C:/q/x", nil},
		{"junction to a drive root", "C:/p/r", "D:/", nil},
		{"junction loop", "C:/p/l1", "", ErrSymlinkLoop},
		{"volume mount point", "C:/p/v", "", fs.ErrNotExist},
		{"under a volume mount point", "C:/p/v/x", "", fs.ErrNotExist},
		{"device target", "C:/p/d", "", fs.ErrNotExist},
		{"symlink to a device", "C:/p/sd", "", fs.ErrNotExist},
		{"irregular non-link", "C:/p/h", "C:/p/h", nil},
		{"irregular, no target", "C:/p/gone", "C:/p/gone", nil},
		{"irregular, target unreadable", "C:/p/denied/x", "", fs.ErrPermission},
		{"symlink, target unreadable", "C:/p/broken/x", "", syscall.EINVAL},
		{"symlink to a junction", "C:/p/s/x", "C:/q/x", nil},
		{"symlink through a chain", "C:/p/mixed", "C:/q/x", nil},
		{"junction to a symlink", "C:/p/through/x", "C:/q/x", nil},
		{"missing", "C:/p/none", "", fs.ErrNotExist},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := evalSymlinksWith(windowsDisk, c.in)
			if c.wantErr == nil {
				if err != nil || got != c.want {
					t.Errorf("evalSymlinksWith(%q) = %q, %v; want %q, nil", c.in, got, err, c.want)
				}
				return
			}
			if !errors.Is(err, c.wantErr) {
				t.Errorf("evalSymlinksWith(%q) err = %v, want %v", c.in, err, c.wantErr)
			}
		})
	}
}

// DECISIONS 419: a device target counts as dangling with an error that names no path, neither the link's nor the target's.
func TestDeviceTargetNamesNoPath(t *testing.T) {
	for _, in := range []string{"C:/p/v", "C:/p/d"} {
		_, err := evalSymlinksWith(windowsDisk, in)
		if err == nil {
			t.Fatalf("evalSymlinksWith(%q) err = nil", in)
		}
		for _, leak := range []string{in, windowsDisk[in].target, "Volume", "PhysicalDrive"} {
			if strings.Contains(err.Error(), leak) {
				t.Errorf("evalSymlinksWith(%q) err %q names %q", in, err, leak)
			}
		}
	}
}

// WIRE.md §6.5, DECISIONS 419: readLink counts ModeSymlink, or ModeIrregular whose target readlink reads; an irregular entry is no link only when readlink says not-exist or EINVAL, any other failure, and any failure on a symlink, an error.
func TestReadLinkModes(t *testing.T) {
	cases := []struct {
		name, child, want string
		isLink            bool
		wantErr           error
	}{
		{"junction", "C:/p/j", "C:/q", true, nil},
		{"symlink", "C:/p/s", "j", true, nil},
		{"placeholder, EINVAL", "C:/p/h", "", false, nil},
		{"irregular, not-exist", "C:/p/gone", "", false, nil},
		{"irregular, permission denied", "C:/p/denied", "", false, fs.ErrPermission},
		{"symlink, readlink fails", "C:/p/broken", "", false, syscall.EINVAL},
		{"directory", "C:/p", "", false, nil},
		{"regular file", "C:/q/x", "", false, nil},
	}
	for _, c := range cases {
		target, isLink, err := readLink(windowsDisk, c.child)
		errOK := (c.wantErr == nil && err == nil) || (c.wantErr != nil && errors.Is(err, c.wantErr))
		if !errOK || isLink != c.isLink || target != c.want {
			t.Errorf("%s: readLink(%q) = %q, %v, %v; want %q, %v, %v", c.name, c.child, target, isLink, err, c.want, c.isLink, c.wantErr)
		}
	}
}

// resolverOf is a recorded resolver over a fixed table, any other name failing.
func resolverOf(real map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if r, ok := real[name]; ok {
			return r, nil
		}
		return "", fs.ErrNotExist
	}
}

// WIRE.md §6.5, DECISIONS 419: a link candidate is ModeSymlink, or ModeIrregular where the FS resolves links.
func TestLinkCandidate(t *testing.T) {
	cases := []struct {
		name string
		fsys FS
		mode fs.FileMode
		want bool
	}{
		{"symlink", OS(), fs.ModeSymlink, true},
		{"symlink, FS without links", noLinks{OS()}, fs.ModeSymlink, true},
		{"irregular", OS(), fs.ModeIrregular, true},
		{"irregular directory", OS(), fs.ModeIrregular | fs.ModeDir, true},
		{"irregular, FS without links", noLinks{OS()}, fs.ModeIrregular, false},
		{"directory", OS(), fs.ModeDir, false},
		{"regular file", OS(), 0, false},
		{"device", OS(), fs.ModeDevice, false},
	}
	for _, c := range cases {
		if got := LinkCandidate(c.fsys, c.mode); got != c.want {
			t.Errorf("%s: LinkCandidate(%v) = %v, want %v", c.name, c.mode, got, c.want)
		}
	}
}

// WIRE.md §6.5, DECISIONS 419: an entry is Unlinked when it resolves to its directory's real path joined with its own name; false when resolving fails.
func TestUnlinked(t *testing.T) {
	resolve := resolverOf(map[string]string{
		"/p":         "/p",
		"/p/h":       "/p/h",
		"/p/j":       "/q",
		"/p/near":    "/p/other",
		"/alias":     "/real",
		"/alias/h":   "/real/h",
		"/alias/j":   "/q",
		"/h":         "/h",
		"/gone/h":    "/real/h",
		"/alias/odd": "/real/h",
	})
	cases := []struct {
		name, dir, entry string
		want             bool
	}{
		{"placeholder", "/p", "h", true},
		{"junction", "/p", "j", false},
		{"link to a sibling", "/p", "near", false},
		{"placeholder under an aliased directory", "/alias", "h", true},
		{"junction under an aliased directory", "/alias", "j", false},
		{"another name in the real directory", "/alias", "odd", false},
		{"under the root", "/", "h", true},
		{"resolving the entry fails", "/p", "none", false},
		{"resolving the directory fails", "/gone", "h", false},
	}
	for _, c := range cases {
		if got := Unlinked(resolve, c.dir, c.entry); got != c.want {
			t.Errorf("%s: Unlinked(%q, %q) = %v, want %v", c.name, c.dir, c.entry, got, c.want)
		}
	}
}
