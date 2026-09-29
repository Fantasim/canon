package workspace

import (
	"io/fs"
	"path"

	"github.com/fantasim/canonlang/internal/build"
)

// writable is the file system under the snapshot, if it can be written.
func (s *snapFS) writable() (build.WriteFS, error) {
	w, ok := s.base.(build.WriteFS)
	if !ok {
		return nil, build.ErrReadOnly
	}
	return w, nil
}

// pin keeps what this snapshot and every older one still in use read at abs and in its
// directory's listing before a writer changes them, so a reader of any of them never sees a
// write that ended after it took its snapshot (API.md S9).
func (s *snapFS) pin(abs string) {
	for _, fs := range s.lineage() {
		fs.get(name{kind: kindFile, abs: abs}, fs.readFile)
		fs.pinName(abs)
	}
}

// pinName pins abs's stat and its directory's listing, what a new directory changes.
func (s *snapFS) pinName(abs string) {
	s.get(name{kind: kindStat, abs: abs}, s.stat)
	s.get(name{kind: kindDir, abs: path.Dir(abs)}, s.readDir)
}

// pinDirs pins every directory MkdirAll(abs) may create: the stat of each missing one and the
// listing above it, up to the first that exists; never a file entry, which a refresh would
// report as a changed file.
func (s *snapFS) pinDirs(abs string) {
	for d := abs; ; d = path.Dir(d) {
		for _, fs := range s.lineage() {
			fs.pinName(d)
		}
		if _, err := s.Stat(d); err == nil || path.Dir(d) == d {
			return
		}
	}
}

func (s *snapFS) WriteFile(abs string, data []byte) error {
	w, err := s.writable()
	if err != nil {
		return err
	}
	s.pin(abs)
	return w.WriteFile(abs, data)
}

func (s *snapFS) Rename(oldname, newname string) error {
	w, err := s.writable()
	if err != nil {
		return err
	}
	s.pin(oldname)
	s.pin(newname)
	return w.Rename(oldname, newname)
}

func (s *snapFS) Remove(abs string) error {
	w, err := s.writable()
	if err != nil {
		return err
	}
	s.pin(abs)
	return w.Remove(abs)
}

func (s *snapFS) MkdirAll(abs string) error {
	w, err := s.writable()
	if err != nil {
		return err
	}
	s.pinDirs(abs)
	return w.MkdirAll(abs)
}

// Chmod sets abs's permissions where the file system under the snapshot can; elsewhere a file
// keeps the mode its write gave it.
func (s *snapFS) Chmod(abs string, mode fs.FileMode) error {
	if m, ok := s.base.(build.ModeFS); ok {
		return m.Chmod(abs, mode)
	}
	return nil
}
