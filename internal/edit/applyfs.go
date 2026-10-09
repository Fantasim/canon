package edit

import (
	"bytes"
	"errors"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/project"
)

// fileState is one path an edit reads or writes: the file there at the base snapshot, the one
// there as edited so far (nil: absent), and the base path its bytes descend from ("" for new ones).
type fileState struct {
	display, abs string
	raw          []byte
	existed      bool
	cur          []byte
	origin       string // the base path cur descends from, through edits and Renames (N8)
	checked      bool   // its layout was judged (M9)
	normalized   bool   // it was not in canonical layout: normalized first (M9)
	fixed        bool   // cur is what Rewrite printed from a fixed point: one too (API.md M5)
	steps        []writeStep
	tree         *jsonsrc.Node // its JSON document, parsed from treeOf
	treeOf       []byte
}

// change is the path's net Change, its file at the base against its final one, whatever the
// operations did between; false when they are the same (log-2026-10-01 M4.1 ruling, N3, N8).
func (s *fileState) change() (Change, bool) {
	switch {
	case s.existed && s.cur == nil:
		return Change{Kind: ChangeDeleted, Path: s.display, Before: s.raw}, true
	case !s.existed && s.cur != nil:
		return Change{Kind: ChangeCreated, Path: s.display, After: s.cur}, true
	case s.existed && !bytes.Equal(s.raw, s.cur):
		return Change{Kind: ChangeModified, Path: s.display, Before: s.raw, After: s.cur}, true
	}
	return Change{}, false
}

// state is the file at display, read from the project's file system on first use.
func (a *applier) state(display string) (*fileState, error) {
	if s, ok := a.files[display]; ok {
		return s, nil
	}
	abs, ok := a.env.Project.Abs(display)
	if !ok {
		return nil, &PathError{Seg: rootSeg, Err: ErrBadPath}
	}
	s := &fileState{display: display, abs: abs}
	data, err := a.env.Project.FS().ReadFile(abs)
	switch {
	case err == nil:
		s.raw, s.cur, s.existed, s.origin = data, slices.Clone(data), true, display
	case !errors.Is(err, fs.ErrNotExist):
		return nil, asIO(err)
	}
	a.files[display] = s
	return s, nil
}

// exists reports a file at display in the current state.
func (a *applier) exists(display string) (bool, error) {
	s, err := a.state(display)
	if err != nil {
		return false, err
	}
	return s.cur != nil, nil
}

// free is nil when no file is at display in the current state, else a *CollisionError (N3).
func (a *applier) free(display string) error {
	taken, err := a.exists(display)
	switch {
	case err != nil:
		return err
	case taken:
		return &CollisionError{Paths: []string{display}}
	}
	return nil
}

// settle analyzes the current state again when an operation changed files since the last
// analysis, over the files as edited (log-2026-09-29 M4, E1 vs E21).
func (a *applier) settle() error {
	if !a.dirty {
		return nil
	}
	an, err := a.env.Project.Over(a.overlay()).AnalyzeOnly(a.ctx, a.selected)
	if err != nil {
		return asIO(err)
	}
	a.snap, a.host, a.dirty = NewSnapshot(an), a.env.Host(an), false
	return nil
}

// overlay is the project's file system with the files as edited so far: each path's final state,
// as the plan's Changes report it.
func (a *applier) overlay() *overlayFS {
	over := &overlayFS{base: a.env.Project.FS(), files: map[string][]byte{}}
	for _, s := range a.files { //canon:unordered fills a map by name
		if s.abs != "" && (s.cur != nil || s.existed) {
			over.files[s.abs] = s.cur
		}
	}
	return over
}

// overlayFS is a file system with some files replaced, created or deleted (nil) in memory.
type overlayFS struct {
	base  project.FS
	files map[string][]byte // by absolute name
}

func (o *overlayFS) ReadFile(name string) ([]byte, error) {
	data, ok := o.files[name]
	switch {
	case !ok:
		return o.base.ReadFile(name)
	case data == nil:
		return nil, notExist(name)
	}
	return slices.Clone(data), nil
}

func (o *overlayFS) Stat(name string) (fs.FileInfo, error) {
	if data, ok := o.files[name]; ok {
		if data == nil {
			return nil, notExist(name)
		}
		return memInfo{name: path.Base(name), size: int64(len(data))}, nil
	}
	info, err := o.base.Stat(name)
	if errors.Is(err, fs.ErrNotExist) && o.holds(name) {
		return memInfo{name: path.Base(name), dir: true}, nil
	}
	return info, err
}

// ReadDir is the base listing with the deleted files taken out and the new files and the
// directories that hold them added, sorted by name.
func (o *overlayFS) ReadDir(name string) ([]fs.DirEntry, error) {
	list, err := o.base.ReadDir(name)
	if errors.Is(err, fs.ErrNotExist) && o.holds(name) {
		list, err = nil, nil
	}
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []fs.DirEntry
	for _, e := range list {
		full := project.Join(name, e.Name())
		if data, ok := o.files[full]; ok && data == nil {
			continue
		}
		seen[e.Name()] = true
		out = append(out, e)
	}
	for _, abs := range slices.Sorted(maps.Keys(o.files)) {
		if child, isDir, ok := o.childOf(name, abs); ok && !seen[child] {
			seen[child] = true
			out = append(out, fs.FileInfoToDirEntry(memInfo{name: child, dir: isDir, size: int64(len(o.files[abs]))}))
		}
	}
	slices.SortFunc(out, func(x, y fs.DirEntry) int { return strings.Compare(x.Name(), y.Name()) })
	return out, nil
}

// childOf is the entry of directory dir that leads to the new file abs, and whether it is a directory.
func (o *overlayFS) childOf(dir, abs string) (string, bool, bool) {
	rest, ok := strings.CutPrefix(abs, strings.TrimSuffix(dir, pathSep)+pathSep)
	if !ok || o.files[abs] == nil {
		return "", false, false
	}
	child, _, deeper := strings.Cut(rest, pathSep)
	return child, deeper, true
}

// holds reports a directory that a new file lies under.
func (o *overlayFS) holds(dir string) bool {
	for abs, data := range o.files { //canon:unordered a membership test
		if data != nil && strings.HasPrefix(abs, strings.TrimSuffix(dir, pathSep)+pathSep) {
			return true
		}
	}
	return false
}

// EvalSymlinks resolves a new file's directory through the base, the file itself being no link; a volume's root ends the walk.
func (o *overlayFS) EvalSymlinks(name string) (string, error) {
	if data, ok := o.files[name]; !ok || data == nil {
		if _, err := o.base.Stat(name); err == nil || !o.holds(name) {
			return project.EvalSymlinks(o.base, name)
		}
	}
	parent := project.DirOf(name)
	if parent == name { // a volume's root the base cannot resolve is its own real path
		return name, nil
	}
	dir, err := o.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	return project.Join(dir, path.Base(name)), nil
}

func notExist(name string) error {
	return &fs.PathError{Op: opRead, Path: name, Err: fs.ErrNotExist}
}

// memInfo is the fs.FileInfo of a file or directory that exists only in memory.
type memInfo struct {
	name string
	size int64
	dir  bool
}

func (m memInfo) Name() string       { return m.name }
func (m memInfo) Size() int64        { return m.size }
func (m memInfo) ModTime() time.Time { return time.Time{} }
func (m memInfo) IsDir() bool        { return m.dir }
func (m memInfo) Sys() any           { return nil }

func (m memInfo) Mode() fs.FileMode {
	if m.dir {
		return fs.ModeDir | dirMode
	}
	return fileMode
}

// netChanges are the plan's file Changes, by path: each path's net change (fileState.change);
// a path Created with the bytes of one Deleted, neither otherwise involved, is reported as one
// Renamed (log-2026-10-01 M4.1 ruling, N8). Each change's writes go to writes, for M6's tests.
func (a *applier) netChanges(writes map[string][]writeStep) []Change {
	byPath := map[string]Change{}
	for _, display := range slices.Sorted(maps.Keys(a.files)) {
		if c, ok := a.files[display].change(); ok {
			byPath[display] = c
			writes[display] = a.files[display].steps
		}
	}
	for _, display := range slices.Sorted(maps.Keys(byPath)) {
		c, s := byPath[display], a.files[display]
		gone, ok := byPath[s.origin]
		if c.Kind != ChangeCreated || !ok || gone.Kind != ChangeDeleted {
			continue
		}
		byPath[display] = Change{Kind: ChangeRenamed, Path: display, OldPath: s.origin, Before: gone.Before, After: c.After}
		delete(byPath, s.origin)
		delete(writes, s.origin)
	}
	out := slices.Collect(maps.Values(byPath))
	slices.SortFunc(out, func(x, y Change) int { return strings.Compare(x.Path, y.Path) })
	return out
}
