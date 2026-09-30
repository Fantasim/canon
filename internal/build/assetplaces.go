package build

import (
	"path"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// assetPlaces keeps, for one run, each asset root and folder placed: placing reads no file, so a
// later asset check of the run asks only the listings again (log-2026-09-29 M4 P18).
type assetPlaces struct {
	roots   map[assetRoot]placedRoot
	folders map[assetFolder]project.Path
}

// assetRoot is an asset root as written, and the directory of the file it is written in.
type assetRoot struct {
	root, from string
}

// placedRoot is an asset root's directory, and whether it resolved.
type placedRoot struct {
	dir project.Path
	ok  bool
}

// assetFolder is a folder segment under a directory an asset walk reached.
type assetFolder struct {
	at  project.Path
	seg string
}

// root is root, written in from, placed through layout; its finding is thrown away (WIRE.md §2.2).
func (p *assetPlaces) root(layout *project.Layout, root, from string) (project.Path, bool) {
	k := assetRoot{root: root, from: from}
	if r, ok := p.roots[k]; ok {
		return r.dir, r.ok
	}
	dir, ok := layout.Resolve(root, from, source.Span{}, diag.NewBag(nil, ""))
	if p.roots == nil {
		p.roots = map[assetRoot]placedRoot{}
	}
	p.roots[k] = placedRoot{dir: dir, ok: ok}
	return dir, ok
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
