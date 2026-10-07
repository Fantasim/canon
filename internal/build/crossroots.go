package build

import (
	"path/filepath"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// crossRoots is E8022: an import of copy e of p under a root placed otherwise relative to e's than declared (CODEGEN.md §2.8).
func (r *run) crossRoots(p *ir.Package, e *ir.Emit) {
	if e.Target != ir.TargetCpp && e.Target != ir.TargetTS || e.Dir == "" {
		return
	}
	at, ok := r.outPath(e)
	if !ok || r.s.layout.Absent(at.Root) {
		return
	}
	declared, _ := project.NewLayout(r.s.proj, r.p.dir, nil, diag.NewBag(nil, p.Name))
	for _, ref := range p.Imports { // what the copy names or reaches: ir's importUse of a code target, as for E8025
		if other, crosses := r.crosses(declared, at, copyOf(ref.Emits, e.Target)); crosses {
			proj := r.s.proj
			diag.E8022.At(r.emitSpan(p, e), at.Display, other.Display, check.RootLabel(proj, at.Root), check.RootLabel(proj, other.Root)).
				Report(r.bags[p.Name])
		}
	}
}

// crosses reports the out of the imported copy o, when it lies under another root than at and
// the two roots are placed otherwise, relative to each other, here than in declared.
func (r *run) crosses(declared *project.Layout, at project.Path, o *ir.Emit) (project.Path, bool) {
	if o == nil {
		return project.Path{}, false
	}
	other, ok := r.outPath(o)
	if !ok || other.Root == at.Root || !r.s.layout.Moved(at.Root) && !r.s.layout.Moved(other.Root) {
		return other, false
	}
	return other, !sameRelative(r.s.layout, declared, at.Root, other.Root)
}

// copyOf is the placed emit of target t among emits, the copy the narrowed package uses; nil when none.
func copyOf(emits []*ir.Emit, t ir.Target) *ir.Emit {
	for _, o := range emits {
		if o.Target == t && o.Dir != "" {
			return o
		}
	}
	return nil
}

// outPath is e's out as this machine places it: its display path and the root it names ("" for the project).
func (r *run) outPath(e *ir.Emit) (project.Path, bool) {
	return r.s.layout.Resolve(e.Out, e.From, source.Span{}, diag.NewBag(nil, ""))
}

// sameRelative reports the directory of root to relative to that of root from (the project's
// for "") the same, lexically, as placed and as declared; false when either has no relative
// path (another volume).
func sameRelative(placed, declared *project.Layout, from, to string) bool {
	here, ok := relativeRoot(placed, from, to)
	if !ok {
		return false
	}
	there, ok := relativeRoot(declared, from, to)
	return ok && here == there
}

// relativeRoot is the directory of root to relative to that of root from in layout l.
func relativeRoot(l *project.Layout, from, to string) (string, bool) {
	rel, err := filepath.Rel(filepath.FromSlash(rootDir(l, from)), filepath.FromSlash(rootDir(l, to)))
	return filepath.ToSlash(rel), err == nil
}

// rootDir is the directory of root in layout l, the project's for "".
func rootDir(l *project.Layout, root string) string {
	if root == "" {
		return l.Dir
	}
	at, _ := l.Resolve(rootMark+root, "", source.Span{}, diag.NewBag(nil, "")) // a declared root resolves
	return at.Abs
}
