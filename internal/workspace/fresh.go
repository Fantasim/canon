package workspace

import (
	"io/fs"
	"slices"

	"github.com/fantasim/canonlang/internal/project"
)

// freshFS is a snapshot's view of the disk as it is now, its overlays on it: each read anew and
// kept nowhere. A build re-reads its inputs through it under its writer locks (DECISIONS 352).
type freshFS struct {
	s *snapFS
}

// Fresh is s's view of the disk as it is now.
func (s *snapFS) Fresh() project.FS { return freshFS{s: s} }

func (f freshFS) ReadFile(abs string) ([]byte, error) {
	e := f.s.readFile(abs)
	return slices.Clone(e.data), e.err
}

func (f freshFS) Stat(abs string) (fs.FileInfo, error) {
	e := f.s.stat(abs)
	return e.info, e.err
}

func (f freshFS) ReadDir(abs string) ([]fs.DirEntry, error) {
	e := f.s.readDir(abs)
	return e.list, e.err
}

// EvalSymlinks resolves links as the snapshot does, anew (WIRE.md §6.5).
func (f freshFS) EvalSymlinks(abs string) (string, error) {
	e := f.s.resolve(abs)
	return string(e.data), e.err
}
