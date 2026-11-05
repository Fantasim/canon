package build

import (
	"path"

	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// WalkBase is the directory a load.dir walk starts at, absolute and there or not, with the
// display path of the source file whose load names it (WIRE.md 6.5).
type WalkBase struct {
	File string
	Dir  string
}

// WalkBases are the walk bases of every load.dir of the snapshot's files, layers included, in package then file order.
func (a *Analysis) WalkBases() []WalkBase {
	s := a.r.s
	var out []WalkBase
	for _, u := range s.units {
		for _, f := range u.Files {
			for _, dir := range walkBasesOf(s.layout, f, s.gen.loadsOf(f)) {
				out = append(out, WalkBase{File: f.Src.Path, Dir: dir})
			}
		}
	}
	return out
}

// FileWalkBases are the walk bases of every load.dir of f, a file parsed outside the snapshot (an edit's new text).
func (a *Analysis) FileWalkBases(f *syntax.File) []string {
	return walkBasesOf(a.r.s.layout, f, walkLoads(f))
}

// walkBasesOf is the base of each load.dir of loads, f's, resolved from f's directory.
func walkBasesOf(layout *project.Layout, f *syntax.File, loads []loadAt) []string {
	from := path.Dir(f.Src.Path)
	var out []string
	for _, at := range loads {
		if dir, ok := load.DirBase(layout, from, at.e); ok {
			out = append(out, dir)
		}
	}
	return out
}
