package project

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// Placement is what places a project's roots on this machine besides project.canon (SPEC §3.1).
type Placement struct {
	Local     *Local            // project.local.canon checked; nil when there is none
	Overrides map[string]string // --root or Options.Roots: FS names, or relative to the project directory
	FS        FS                // where presence is judged
}

// origin is where a root's directory on this machine comes from (E1013's variants).
type origin int

// placed is how a root was placed: by whom, the path as written there, and where it says so.
type placed struct {
	origin  origin
	written string
	span    source.Span
}

// Place lays p's roots out under dir as this machine does, root by root: the overrides over
// project.local.canon over project.canon, and judges which are present. An absent required
// root outside the project is E1013 and false (API.md O2, O3, DECISIONS 332).
func Place(p *Project, dir string, pl Placement, bag *diag.Bag) (*Layout, bool) {
	l := declaredLayout(p, dir)
	from := make(map[string]placed, len(p.Roots))
	for _, r := range p.Roots {
		from[r.Name] = placed{origin: fromDeclared, written: r.Path, span: r.Span}
	}
	if pl.Local != nil {
		for _, r := range pl.Local.Roots {
			l.dirs[r.Name] = l.abs(HostPaths().FromAPI(r.Path, l.Dir))
			from[r.Name] = placed{origin: fromLocal, written: r.Path, span: r.Span}
		}
	}
	if !l.override(pl.Overrides, from, bag) {
		return l, false
	}
	ok := true
	for _, r := range p.Roots {
		if l.present(pl.FS, r.Name) {
			continue
		}
		if r.Optional {
			l.absent[r.Name] = true
			continue
		}
		f := from[r.Name]
		absentRoot[f.origin](f.span, r.Name, f.written).Report(bag)
		ok = false
	}
	return l, ok
}

// present reports root's directory at or inside the project directory, judged on the text, or
// a directory fsys finds, a symbolic link to one included (DECISIONS 332).
func (l *Layout) present(fsys FS, root string) bool {
	dir := l.dirs[root]
	if dir == l.Dir || strings.HasPrefix(dir, strings.TrimSuffix(l.Dir, sep)+sep) {
		return true
	}
	info, err := fsys.Stat(dir)
	return err == nil && info.IsDir()
}

// Absent reports root an optional root this machine does not have: what reads it fails at the
// value, what writes into it is skipped (DECISIONS 332). False for a required or undeclared root.
func (l *Layout) Absent(root string) bool {
	return l.absent[root]
}

// Moved reports root placed elsewhere than project.canon places it, by project.local.canon or
// an override (DECISIONS 332: a relative include between two moved roots is E8022).
func (l *Layout) Moved(root string) bool {
	dir, ok := l.dirs[root]
	return ok && dir != l.declared[root]
}
