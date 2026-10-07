package workspace

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/fantasim/canonlang/internal/project"
)

// SetOverlay replaces file's content in every read and in the revision, a writer (API.md §3.4).
func (p *Project) SetOverlay(file string, content []byte) error {
	return p.SetOverlayContext(context.Background(), file, content)
}

// SetOverlayContext is SetOverlay waiting for the write lock only until ctx is done, then
// returning ctx.Err() (API.md S9, S11).
func (p *Project) SetOverlayContext(ctx context.Context, file string, content []byte) error {
	return p.overlay(ctx, file, func(over map[string][]byte, abs string) bool {
		if old, ok := over[abs]; ok && slices.Equal(old, content) {
			return false
		}
		over[abs] = append(make([]byte, 0, len(content)), content...) // never nil: nil is a file gone
		return true
	})
}

// ClearOverlay removes file's overlay, if it has one (API.md §3.4). It is a writer (S9).
func (p *Project) ClearOverlay(file string) error {
	return p.ClearOverlayContext(context.Background(), file)
}

// ClearOverlayContext is ClearOverlay waiting for the write lock only until ctx is done (S11).
func (p *Project) ClearOverlayContext(ctx context.Context, file string) error {
	return p.overlay(ctx, file, func(over map[string][]byte, abs string) bool {
		_, ok := over[abs]
		delete(over, abs)
		return ok
	})
}

// overlay applies change to the overlays at file and publishes the snapshot it gives, where the
// file and the directories above it are read again.
func (p *Project) overlay(ctx context.Context, file string, change func(map[string][]byte, string) bool) error {
	if err := p.lock(ctx); err != nil {
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

// dropAt marks for a fork the entries an overlay at abs changes: abs's content, and the stat,
// real path and listing of abs and of each directory above it.
func dropAt(dropped map[name]*entry, abs string) {
	dropped[name{kind: kindFile, abs: abs}] = nil
	for d := abs; ; d = project.DirOf(d) {
		dropped[name{kind: kindStat, abs: d}] = nil
		dropped[name{kind: kindLink, abs: d}] = nil
		dropped[name{kind: kindDir, abs: project.DirOf(d)}] = nil
		if project.DirOf(d) == d {
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

// overlayResolve is abs's real path when an overlay makes it, a file or a directory above one the
// disk lacks: its parent's real path and its name; a file an overlay deletes does not resolve.
// False for any other name, which the disk resolves (API.md E18, V13).
func (s *snapFS) overlayResolve(abs string) (*entry, bool) {
	data, over := s.over[abs]
	switch {
	case over && data == nil:
		return gone(abs), true
	case !over && !s.holdsOverlay(abs):
		return nil, false
	}
	parent := project.DirOf(abs)
	// The disk, not the snapshot's entries, which hold the overlay; asked once, the answer kept as abs's link entry (S1).
	if _, err := s.base.Stat(abs); err == nil || parent == abs || s.linkOnDisk(parent, path.Base(abs)) {
		return nil, false // a dangling link resolves through its text (API.md S12)
	}
	up := s.get(name{kind: kindLink, abs: parent}, s.resolve)
	if up.err != nil {
		return up, true
	}
	e := &entry{data: []byte(project.Join(string(up.data), path.Base(abs))), over: true}
	e.sum = sum{class: classOK, hash: sha256.Sum256(e.data)}
	return e, true
}

// throughLink is the real path of abs, a link dangling on the disk, when an overlay makes its
// target: a load.dir base an edit in memory brings to life (API.md E17, E18).
func (s *snapFS) throughLink(abs string) (*entry, bool) {
	text, err := s.Readlink(abs)
	if err != nil {
		return nil, false
	}
	target := text
	if !path.IsAbs(text) {
		target = project.Join(project.DirOf(abs), text)
	}
	if data, over := s.over[target]; (!over || data == nil) && !s.holdsOverlay(target) {
		return nil, false
	}
	return s.get(name{kind: kindLink, abs: target}, s.resolve), true
}

// unresolved is every name s failed to resolve to a real path, as a link dangling there does.
func (s *snapFS) unresolved() []name {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []name
	//canon:unordered each name is dropped on its own
	for n, e := range s.ents {
		if n.kind == kindLink && e.err != nil {
			out = append(out, n)
		}
	}
	return out
}

// linkOnDisk reports base a symbolic link in the disk's listing of dir.
func (s *snapFS) linkOnDisk(dir, base string) bool {
	list, err := s.base.ReadDir(dir)
	return err == nil && slices.ContainsFunc(list, func(e fs.DirEntry) bool {
		return e.Name() == base && e.Type()&fs.ModeSymlink != 0
	})
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
