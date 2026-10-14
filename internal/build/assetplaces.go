package build

import (
	"errors"
	"path"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// assetPlaces keeps, for one run, each asset root and folder placed: placing reads no file, so a
// later asset check of the run asks only the listings again (log-2026-09-29 M4 P18).
type assetPlaces struct {
	roots   map[assetRoot]placedRoot
	folders map[assetFolder]project.Path
	bounds  load.Bounds // where a followed link must lead, computed on first use (WIRE.md §6.5)
	bounded bool
}

// bound is load.dir's link bounds for layout, resolved through fsys once per run (TYPES.md §13.4, DECISIONS 376).
func (p *assetPlaces) bound(fsys project.FS, layout *project.Layout) load.Bounds {
	if !p.bounded {
		p.bounds, p.bounded = load.LinkBounds(fsys, layout), true
	}
	return p.bounds
}

// assetRoot is an asset root as written, and the directory of the file it is written in.
type assetRoot struct {
	root, from string
}

// placedRoot is an asset root's directory, whether it resolved, and the first link of its path
// that leads outside the bounds, when one does.
type placedRoot struct {
	dir     project.Path
	ok      bool
	blocked string
}

// assetFolder is a folder segment under a directory an asset walk reached.
type assetFolder struct {
	at  project.Path
	seg string
}

// root is root, written in from, placed through layout, its finding thrown away, and named by its
// real path, links resolved through fsys as load.dir resolves them (TYPES.md 13.4, WIRE.md 2.2).
func (p *assetPlaces) root(fsys project.FS, layout *project.Layout, root, from string) (project.Path, bool) {
	k := assetRoot{root: root, from: from}
	if r, ok := p.roots[k]; ok {
		return r.dir, r.ok
	}
	dir, ok := layout.Resolve(root, from, source.Span{}, diag.NewBag(nil, ""))
	placed := placedRoot{dir: dir, ok: ok}
	if ok && !layout.Absent(dir.Root) { // an absent root is never touched (DECISIONS 332)
		placed = p.resolved(fsys, layout, placed)
	}
	if p.roots == nil {
		p.roots = map[assetRoot]placedRoot{}
	}
	p.roots[k] = placed
	return placed.dir, placed.ok
}

// resolved is r named by its real path, a looping root dropped, an outside one blocked (TYPES.md §13.4, DECISIONS 376).
func (p *assetPlaces) resolved(fsys project.FS, layout *project.Layout, r placedRoot) placedRoot {
	real, err := project.EvalSymlinks(fsys, r.dir.Abs)
	switch {
	case errors.Is(err, project.ErrSymlinkLoop):
		r.ok = false
	case err == nil:
		if real != r.dir.Abs && !p.bound(fsys, layout).Within(real) {
			r.blocked = load.FirstLinked(fsys, layout, r.dir)
		}
		r.dir.Abs = real
	}
	return r
}

// blockedRoot is the first link of root's path that leads outside the bounds, "" when none does.
func (p *assetPlaces) blockedRoot(root, from string) string {
	return p.roots[assetRoot{root: root, from: from}].blocked
}

// folder is the folder seg under at, as a display and a file-system name (TYPES.md §13.4).
func (p *assetPlaces) folder(at project.Path, seg string) project.Path {
	k := assetFolder{at: at, seg: seg}
	if f, ok := p.folders[k]; ok {
		return f
	}
	f := project.Path{Display: path.Join(at.Display, seg), Abs: project.Join(at.Abs, seg)}
	if p.folders == nil {
		p.folders = map[assetFolder]project.Path{}
	}
	p.folders[k] = f
	return f
}
