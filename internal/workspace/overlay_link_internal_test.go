package workspace

import (
	"errors"
	"io/fs"
	"path"
	"testing"
	"time"
)

// The disk junctionFS models: the directory /p/d, whose entry pl, of the mode a row sets, has no
// stat; resolved, pl dangles or is itself.
const (
	junctionDir   = "/p/d"
	junctionEntry = "/p/d/pl"
	junctionReal  = "/q/d" // junctionDir's real path, where a row moves it
)

// junctionFS is a disk holding junctionEntry, an entry of mode whose real path is itself when it
// resolves, as a cloud placeholder's is; otherwise it dangles, as a junction to nothing does.
// moved resolves junctionDir to junctionReal.
type junctionFS struct {
	mode     fs.FileMode
	resolves bool
	moved    bool
}

// modeInfo is the stat of a listing entry of one mode.
type modeInfo struct {
	name string
	mode fs.FileMode
}

func (m modeInfo) Name() string       { return m.name }
func (m modeInfo) Size() int64        { return 0 }
func (m modeInfo) ModTime() time.Time { return time.Time{} }
func (m modeInfo) IsDir() bool        { return m.mode.IsDir() }
func (m modeInfo) Sys() any           { return nil }
func (m modeInfo) Mode() fs.FileMode  { return m.mode }

func (j junctionFS) ReadFile(name string) ([]byte, error) {
	return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
}

func (j junctionFS) Stat(name string) (fs.FileInfo, error) {
	if name == junctionEntry {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	return modeInfo{name: path.Base(name), mode: fs.ModeDir}, nil
}

func (j junctionFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != junctionDir {
		return nil, nil
	}
	return []fs.DirEntry{fs.FileInfoToDirEntry(modeInfo{name: path.Base(junctionEntry), mode: j.mode})}, nil
}

func (j junctionFS) EvalSymlinks(name string) (string, error) {
	switch {
	case name == junctionEntry && !j.resolves:
		return "", &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrNotExist}
	case name == junctionDir && j.moved:
		return junctionReal, nil
	}
	return name, nil
}

// WIRE.md §6.5, DECISIONS 419: an overlay below a dangling link or junction dangles; below an Unlinked entry, it is the overlay's directory.
func TestOverlayBelowJunction(t *testing.T) {
	for _, c := range []struct {
		name string
		fsys junctionFS
		want string // the real path of junctionEntry; "" when it dangles
	}{
		{"symbolic link", junctionFS{mode: fs.ModeSymlink}, ""},
		{"junction", junctionFS{mode: fs.ModeIrregular}, ""},
		// The link branch would give the disk's answer, junctionEntry itself.
		{"placeholder", junctionFS{mode: fs.ModeIrregular, resolves: true, moved: true}, junctionReal + "/pl"},
		{"directory", junctionFS{mode: fs.ModeDir, resolves: true}, junctionEntry},
	} {
		s := newSnapFS(c.fsys, map[string][]byte{junctionEntry + "/new.json": []byte("{}\n")}, time.Now)
		real, err := s.EvalSymlinks(junctionEntry)
		if dangling := errors.Is(err, fs.ErrNotExist); dangling != (c.want == "") || real != c.want {
			t.Errorf("WIRE.md §6.5, DECISIONS 419: %s: %s resolves to %q, %v; want %q", c.name, junctionEntry, real, err, c.want)
		}
	}
}
