package load

import (
	"io/fs"
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// walker is one load.dir glob's directory walk: bounds a symlink's target must resolve inside,
// onPath the resolved path of every directory currently being recursed into.
type walker struct {
	l      *Loader
	bounds []string
	onPath map[string]bool
	from   string
	span   source.Span
	bag    *diag.Bag
}

// hit is one matched file: its path relative to the walk's base, and its resolved path (WIRE.md §2.3).
type hit struct {
	rel, real string
}

// newWalker starts a walk under base, its bounds resolved once (WIRE.md §2.2, §6.5).
func (l *Loader) newWalker(base project.Path, req Request) *walker {
	return &walker{
		l: l, bounds: l.walkBounds(), onPath: map[string]bool{},
		from: base.Display, span: req.Span, bag: req.Bag,
	}
}

// walkBounds is where a followed symlink's target must resolve: the project directory or any
// declared root, each resolved as a target is (meta/decisions/log-2026-09-24.md "load.dir round 3").
func (l *Loader) walkBounds() []string {
	dirs := append([]string{l.Layout.Dir}, l.Layout.RootDirs()...)
	for i, d := range dirs {
		dirs[i] = l.realOr(d)
	}
	return dirs
}

// realOr is name resolved through the project's FS, or name itself when that FS cannot resolve
// it: a bound or base that does not exist holds no link target, and one without links is itself.
func (l *Loader) realOr(name string) string {
	if real, err := project.EvalSymlinks(l.FS, name); err == nil {
		return real
	}
	return name
}

// within is whether real, a resolved '/'-separated path, is one of bounds or under one.
func within(real string, bounds []string) bool {
	for _, b := range bounds {
		if real == b || strings.HasPrefix(real, strings.TrimSuffix(b, sepStr)+sepStr) {
			return true
		}
	}
	return false
}

// reportSkipped is W7115 for link name in directory rel, broken ones too (log "load.dir round 3").
func (w *walker) reportSkipped(rel, name string) {
	diag.W7115.At(w.span, path.Join(w.from, rel, name)).Report(w.bag)
}

// descend recurses into a directory resolved at childReal, silently not when already on the walk path.
func (w *walker) descend(childReal string, run func() ([]hit, error)) ([]hit, error) {
	if w.onPath[childReal] {
		return nil, nil
	}
	w.onPath[childReal] = true
	defer delete(w.onPath, childReal)
	return run()
}

// entryKind is what one directory entry is for globbing: ok false for a skipped link; real is
// its resolved path, for the next level, a cycle check and deduplication.
type entryKind struct {
	isFile, isDir bool
	real          string
	ok            bool
}

// classify is entry e's kind in the directory rel resolved at real; a symlink is followed only
// when the project's FS resolves it inside w's bounds, any other link skipped.
func (w *walker) classify(rel, real string, e fs.DirEntry) entryKind {
	child := path.Join(real, e.Name())
	if e.Type()&fs.ModeSymlink == 0 {
		return entryKind{isFile: e.Type().IsRegular(), isDir: e.IsDir(), real: child, ok: true}
	}
	target, err := project.EvalSymlinks(w.l.FS, child)
	if err != nil || !within(target, w.bounds) {
		w.reportSkipped(rel, e.Name())
		return entryKind{}
	}
	info, err := w.l.FS.Stat(target)
	if err != nil {
		w.reportSkipped(rel, e.Name())
		return entryKind{}
	}
	return entryKind{isFile: info.Mode().IsRegular(), isDir: info.IsDir(), real: target, ok: true}
}
