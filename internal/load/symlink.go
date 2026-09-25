package load

import (
	"errors"
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

// newWalker starts a walk under base, bounds already resolved once by the caller (WIRE.md §2.2, §6.5).
func (l *Loader) newWalker(base project.Path, bounds []string, req Request) *walker {
	return &walker{
		l: l, bounds: bounds, onPath: map[string]bool{},
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

// reportSkipped is W7115 for link name in directory rel, its variant chosen by the caller
// (log "load.dir round 3").
func (w *walker) reportSkipped(rel, name string, at func(source.Span, string) *diag.Builder) {
	at(w.span, path.Join(w.from, rel, name)).Report(w.bag)
}

// linkCause is W7115's variant for err, project.EvalSymlinks's own result, chosen by errors.Is, never OS text; anything else, an unresolvable FS included, falls to statFailed.
func linkCause(err error) func(source.Span, string) *diag.Builder {
	switch {
	case errors.Is(err, project.ErrSymlinkLoop):
		return diag.W7115.AtLooping
	case errors.Is(err, fs.ErrNotExist):
		return diag.W7115.AtDangling
	default:
		return diag.W7115.AtStatFailed
	}
}

// resolveBase is base's directory, base.Abs itself when no link changed it: outside every bound it is W7115 with an empty match; a dangling or looping link is an error instead, E7004 at the caller.
func (l *Loader) resolveBase(base project.Path, bounds []string, req Request) (real string, matched bool, err error) {
	real, evalErr := project.EvalSymlinks(l.FS, base.Abs)
	switch {
	case evalErr == nil && (real == base.Abs || within(real, bounds)):
		return real, true, nil
	case evalErr == nil:
		diag.W7115.AtOutsideRoots(req.Span, l.firstLinkedSegment(base)).Report(req.Bag)
		return "", false, nil
	case errors.Is(evalErr, fs.ErrNotExist), errors.Is(evalErr, project.ErrSymlinkLoop):
		return "", false, evalErr
	default:
		return base.Abs, true, nil
	}
}

// firstLinkedSegment is base's own written prefix up to and including the first segment itself a link (its real prefix differs from its resolved parent's plus its own text, an aliased ancestor told apart that way), base.Display when the base itself is that link.
func (l *Loader) firstLinkedSegment(base project.Path) string {
	displaySegs := strings.Split(strings.TrimSuffix(base.Display, sepStr), sepStr)
	ownStart := 0
	if base.Root != "" {
		ownStart = 1
	}
	own := displaySegs[ownStart:]
	absSegs := strings.Split(strings.TrimSuffix(base.Abs, sepStr), sepStr)
	if len(own) == 0 || len(own) > len(absSegs) {
		return base.Display
	}
	head := len(absSegs) - len(own)
	realParent := l.realOr(strings.Join(absSegs[:head], sepStr))
	for i, seg := range own {
		real := l.realOr(strings.Join(absSegs[:head+i+1], sepStr))
		if real == realParent+sepStr+seg {
			realParent = real
			continue
		}
		if i == len(own)-1 {
			return base.Display
		}
		return strings.Join(displaySegs[:ownStart+i+1], sepStr) + sepStr
	}
	return base.Display
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
	switch {
	case err != nil:
		w.reportSkipped(rel, e.Name(), linkCause(err))
		return entryKind{}
	case !within(target, w.bounds):
		w.reportSkipped(rel, e.Name(), diag.W7115.AtOutsideRoots)
		return entryKind{}
	}
	info, err := w.l.FS.Stat(target)
	if err != nil {
		w.reportSkipped(rel, e.Name(), diag.W7115.AtStatFailed)
		return entryKind{}
	}
	return entryKind{isFile: info.Mode().IsRegular(), isDir: info.IsDir(), real: target, ok: true}
}
