package workspace

import (
	"io/fs"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
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
// directory's listing before a writer changes them (API.md S9); each name is read once, by
// this snapshot, and an older one that has not read it takes the same entry.
func (s *snapFS) pin(abs string) {
	s.noteWritten(abs)
	older := s.lineage()[1:]
	for _, p := range [...]pinned{
		{n: name{kind: kindFile, abs: abs}, load: (*snapFS).readFile},
		{n: name{kind: kindStat, abs: abs}, load: (*snapFS).stat},
		{n: name{kind: kindDir, abs: project.DirOf(abs)}, load: (*snapFS).readDir},
	} {
		e := s.get(p.n, func(a string) *entry { return p.load(s, a) })
		for _, fs := range older {
			fs.adopt(p, e, s)
		}
	}
}

// noteWritten keeps abs among the names a writer changes through s (takeWritten).
func (s *snapFS) noteWritten(abs string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.written = append(s.written, abs)
}

// takeWritten is every name a writer pinned through s since the last call, forgotten.
func (s *snapFS) takeWritten() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.written
	s.written = nil
	return out
}

// pinned is a name a writer pins, with the method that reads it.
type pinned struct {
	n    name
	load func(*snapFS, string) *entry
}

// pinName pins abs's stat and its directory's listing, what a new directory changes.
func (s *snapFS) pinName(abs string) {
	s.get(name{kind: kindStat, abs: abs}, s.stat)
	s.get(name{kind: kindDir, abs: project.DirOf(abs)}, s.readDir)
}

// adopt stores the writer w's entry e for p, unless s has one; where overlays make the two
// snapshots read differently, s reads p itself.
func (s *snapFS) adopt(p pinned, e *entry, w *snapFS) {
	if len(s.over) > 0 || len(w.over) > 0 {
		s.get(p.n, func(a string) *entry { return p.load(s, a) })
		return
	}
	s.get(p.n, func(string) *entry { return e })
}

// pinDirs pins every directory MkdirAll(abs) may create: the stat of each missing one and the
// listing above it, up to the first that exists; never a file entry, which a refresh would
// report as a changed file.
func (s *snapFS) pinDirs(abs string) {
	for d := abs; ; d = project.DirOf(d) {
		for _, fs := range s.lineage() {
			fs.pinName(d)
		}
		if _, err := s.Stat(d); err == nil || project.DirOf(d) == d {
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

// SyncDir syncs abs's entries where the file system under the snapshot can, as Chmod forwards
// (log-2026-09-29 M4 U8-r); elsewhere it does nothing.
func (s *snapFS) SyncDir(abs string) error {
	if d, ok := s.base.(build.DirSyncer); ok {
		return d.SyncDir(abs)
	}
	return nil
}

// Chmod sets abs's permissions where the file system under the snapshot can; elsewhere a file
// keeps the mode its write gave it.
func (s *snapFS) Chmod(abs string, mode fs.FileMode) error {
	if m, ok := s.base.(build.ModeFS); ok {
		return m.Chmod(abs, mode)
	}
	return nil
}

// Readlink is the text of the link abs on the disk under the snapshot (build.LinkReader), read
// now: only a dangling link's real path is asked of it, never a content.
func (s *snapFS) Readlink(abs string) (string, error) {
	if r, ok := s.base.(build.LinkReader); ok {
		return r.Readlink(abs)
	}
	return "", errNoLinks
}

// settle makes s, an edit planned in memory (API.md E18), the snapshot of what was written (S10)
// before any call reads it: planned files, returned, keep their content until compared with the
// disk (S1); their stats are read again; user are the only overlays left (S12).
func (s *snapFS) settle(user map[string][]byte) []name {
	s.mu.Lock()
	defer s.mu.Unlock()
	var files []name
	//canon:unordered each entry is settled on its own; the caller sorts the files
	for n, e := range s.ents {
		switch {
		case !e.over || n.kind == kindFile && !s.mine[n.abs]:
			continue
		case n.kind == kindFile:
			s.ents[n] = &entry{sum: e.sum, data: e.data, err: e.err} // no stamp: read again at the next refresh
			files = append(files, n)
		default:
			delete(s.ents, n)
		}
		s.log = append(s.log, n)
	}
	s.over, s.mine = user, nil
	s.gen++
	return files
}
