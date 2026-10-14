package build

import (
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/source"
)

// seenDir is the first spelling of a directory the declared outputs pass through, how a finding
// names it, and the emit that declared it (nil: a package's directory).
type seenDir struct {
	dir, display string
	e            *ir.Emit
}

// dirCase is E8152 `dirCase` at each selected emit whose declared directory equals, case aside, an earlier emit's or a package's, or one above it (WIRE.md §8.1, DECISIONS 377).
func (r *run) dirCase() {
	seen := map[string]seenDir{}
	for _, lock := range lockPaths(r.s.names) {
		d := path.Dir(lock)
		seen[strings.ToLower(d)] = seenDir{dir: d, display: d}
	}
	for _, p := range r.ir {
		for _, e := range p.Emits {
			first, shown, clash := r.emitDirCase(e, seen)
			if !clash {
				continue
			}
			diag.E8152.AtDirCase(r.emitSpan(p, e), first.display, shown).Report(r.bags[p.Name])
			r.writes().dirCased[e] = true // both sides write nothing, a view after errors included
			if first.e != nil {
				r.writes().dirCased[first.e] = true
			}
		}
	}
}

// emitDirCase records the declared directories of e canon may create, top-down, up to the first
// seen under another spelling, which it returns with e's display of it.
func (r *run) emitDirCase(e *ir.Emit, seen map[string]seenDir) (seenDir, string, bool) {
	if e.Dir == "" {
		return seenDir{}, "", false
	}
	display := r.declaredDisplay(e)
	for _, d := range r.declaredCreatable(e.Dir) {
		shown, ok := dirDisplay(display, e.Dir, d)
		if !ok {
			shown = d
		}
		first, met := seen[strings.ToLower(d)]
		switch {
		case !met:
			seen[strings.ToLower(d)] = seenDir{dir: d, display: shown, e: e}
		case first.dir != d:
			return first, shown, true
		}
	}
	return seenDir{}, "", false
}

// declaredDisplay is the display path of e's directory where project.canon places the roots.
func (r *run) declaredDisplay(e *ir.Emit) string {
	at, _ := r.declaredLayout().Resolve(e.Out, e.From, source.Span{}, diag.NewBag(nil, "")) // resolves: stage E placed e.Dir from it
	display := strings.TrimSuffix(at.Display, pathSep)
	if e.FileName != "" {
		display = path.Dir(display)
	}
	return display
}

// declaredCreatable are the directories on dir's path, roots as declared, canon may create, top-down: inside the project, or strictly below a root outside it (CODEGEN.md §2.4).
func (r *run) declaredCreatable(dir string) []string {
	var outside []string
	for _, root := range r.declaredLayout().RootDirs() {
		if !inProjectDeclared(root) {
			outside = append(outside, root)
		}
	}
	var out []string
	for d := dir; d != listingMark && d != path.Dir(d); d = path.Dir(d) {
		if inProjectDeclared(d) || slices.ContainsFunc(outside, func(root string) bool {
			rel, in := under(root, d)
			return in && rel != listingMark
		}) {
			out = append(out, d)
		}
	}
	slices.Reverse(out)
	return out
}

// inProjectDeclared reports a project-relative path inside the project, not its own directory.
func inProjectDeclared(d string) bool {
	return d != listingMark && !path.IsAbs(d) && d != parentSeg && !strings.HasPrefix(d, parentSeg+pathSep)
}
