package workspace

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"time"
)

// SetOverlay replaces file's content in every read and in the revision, a writer (API.md §3.4).
func (p *Project) SetOverlay(file string, content []byte) error {
	return p.overlay(file, func(over map[string][]byte, abs string) bool {
		if old, ok := over[abs]; ok && slices.Equal(old, content) {
			return false
		}
		over[abs] = append(make([]byte, 0, len(content)), content...) // never nil: nil is a file gone
		return true
	})
}

// ClearOverlay removes file's overlay, if it has one (API.md §3.4). It is a writer (S9).
func (p *Project) ClearOverlay(file string) error {
	return p.overlay(file, func(over map[string][]byte, abs string) bool {
		_, ok := over[abs]
		delete(over, abs)
		return ok
	})
}

// overlay applies change to the overlays at file and publishes the snapshot it gives, where the
// file and the directories above it are read again.
func (p *Project) overlay(file string, change func(map[string][]byte, string) bool) error {
	if err := p.lock(context.Background()); err != nil {
		return err
	}
	defer p.unlock()
	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()
	s, _, err := p.current()
	if err != nil {
		return err
	}
	abs, ok := s.b.Abs(file)
	if !ok {
		return fmt.Errorf(fmtBadPath, ErrBadPath, file)
	}
	over := maps.Clone(s.fs.over)
	if over == nil {
		over = map[string][]byte{}
	}
	if !change(over, abs) {
		return nil
	}
	dropped := map[name]*entry{}
	dropAt(dropped, abs)
	next := p.snapshot(s.fs.fork(over, dropped))
	p.publish(next, CauseOverlay, []string{next.display(abs)})
	return nil
}

// dropAt marks for a fork the entries an overlay at abs changes: abs's content, and the stat and
// listing of abs and of each directory above it.
func dropAt(dropped map[name]*entry, abs string) {
	dropped[name{kind: kindFile, abs: abs}] = nil
	for d := abs; ; d = path.Dir(d) {
		dropped[name{kind: kindStat, abs: d}] = nil
		dropped[name{kind: kindDir, abs: path.Dir(d)}] = nil
		if path.Dir(d) == d {
			return
		}
	}
}

// gone is the entry of a file an overlay deletes, for a snapshot with an edit applied in memory.
func gone(abs string) *entry {
	err := fmt.Errorf(fmtBadPath, fs.ErrNotExist, abs)
	return &entry{err: err, sum: sum{class: classMissing}, over: true}
}

// overlayStat is abs's stat when an overlay makes it: the overlay itself, or a directory above
// one that the disk does not have.
func (s *snapFS) overlayStat(abs string) (fs.FileInfo, bool) {
	if data, ok := s.over[abs]; ok {
		return overlayInfo{name: path.Base(abs), size: int64(len(data))}, true
	}
	if !s.holdsOverlay(abs) {
		return nil, false
	}
	if _, err := s.base.Stat(abs); err == nil {
		return nil, false
	}
	return overlayInfo{name: path.Base(abs), dir: true}, true
}

// holdsOverlay reports a file an overlay makes below dir.
func (s *snapFS) holdsOverlay(dir string) bool {
	prefix := strings.TrimSuffix(dir, pathSep) + pathSep
	for abs, data := range s.over {
		if data != nil && strings.HasPrefix(abs, prefix) {
			return true
		}
	}
	return false
}

// withOverlays is list less the files an overlay deletes and, for every other overlay below
// dir, the entry of dir that leads to it, when list does not hold one of that name.
func (s *snapFS) withOverlays(dir string, list []fs.DirEntry) []fs.DirEntry {
	prefix := strings.TrimSuffix(dir, pathSep) + pathSep
	out := slices.DeleteFunc(slices.Clone(list), func(e fs.DirEntry) bool {
		data, ok := s.over[prefix+e.Name()]
		return ok && data == nil
	})
	for _, abs := range slices.Sorted(maps.Keys(s.over)) {
		rel, ok := strings.CutPrefix(abs, prefix)
		if !ok || s.over[abs] == nil {
			continue
		}
		child, _, deeper := strings.Cut(rel, pathSep)
		if slices.ContainsFunc(out, func(e fs.DirEntry) bool { return e.Name() == child }) {
			continue
		}
		info := overlayInfo{name: child, dir: deeper}
		if !deeper {
			info.size = int64(len(s.over[abs]))
		}
		out = append(out, fs.FileInfoToDirEntry(info))
	}
	return out
}

// overlayInfo is the stat of an overlay, a regular file of its content's size, or of a
// directory that holds one.
type overlayInfo struct {
	name string
	size int64
	dir  bool
}

func (o overlayInfo) Name() string       { return o.name }
func (o overlayInfo) Size() int64        { return o.size }
func (o overlayInfo) ModTime() time.Time { return time.Time{} }
func (o overlayInfo) IsDir() bool        { return o.dir }
func (o overlayInfo) Sys() any           { return nil }

func (o overlayInfo) Mode() fs.FileMode {
	if o.dir {
		return fs.ModeDir
	}
	return 0
}
