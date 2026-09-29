package workspace

import (
	"io/fs"
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// throughFS is the disk as an edit's commit sees it (API.md N9, N11, S9): it reads the disk
// itself, never a snapshot's copy, and writes through the snapshot, which first keeps what its
// readers read; the journal's directory, which no snapshot reads, it writes directly.
type throughFS struct {
	snap    *snapFS
	disk    build.WriteFS
	journal string // the journal's directory (API.md N10)
}

// through is s's writes over the disk w, reads from w.
func through(s *snapFS, w build.WriteFS) throughFS {
	return throughFS{snap: s, disk: w, journal: path.Join(s.root, edit.JournalDir)}
}

// to is where a write of names goes: the disk for the journal's directory, else the snapshot.
func (t throughFS) to(names ...string) build.WriteFS {
	for _, n := range names {
		if n != t.journal && !strings.HasPrefix(n, t.journal+pathSep) {
			return t.snap
		}
	}
	return t.disk
}

func (t throughFS) ReadFile(name string) ([]byte, error) { return t.disk.ReadFile(name) }

func (t throughFS) Stat(name string) (fs.FileInfo, error) { return t.disk.Stat(name) }

func (t throughFS) ReadDir(name string) ([]fs.DirEntry, error) { return t.disk.ReadDir(name) }

func (t throughFS) WriteFile(name string, data []byte) error {
	return t.to(name).WriteFile(name, data)
}

func (t throughFS) Rename(oldname, newname string) error {
	return t.to(oldname, newname).Rename(oldname, newname)
}

func (t throughFS) Remove(name string) error { return t.to(name).Remove(name) }

func (t throughFS) MkdirAll(name string) error { return t.to(name).MkdirAll(name) }

// SyncDir syncs where the disk can (edit's optional capability, ADR-0010).
func (t throughFS) SyncDir(dir string) error { return t.snap.SyncDir(dir) }

// Chmod sets permissions where the disk can.
func (t throughFS) Chmod(name string, mode fs.FileMode) error { return t.snap.Chmod(name, mode) }

// EvalSymlinks resolves links on the disk; a disk that has none resolves every name to itself.
func (t throughFS) EvalSymlinks(name string) (string, error) {
	r, ok := t.disk.(interface {
		EvalSymlinks(name string) (string, error)
	})
	if !ok {
		return name, nil
	}
	return r.EvalSymlinks(name)
}
