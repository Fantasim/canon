package build

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// Facts is project.canon read and checked, with every declared root placed (API.md §5.5).
type Facts struct {
	Project *project.Project
	Roots   []PlacedRoot
}

// PlacedRoot is a declared root as this machine places it, its Dir never resolved through links (DECISIONS 346).
type PlacedRoot struct {
	Name   string
	Dir    string
	Absent bool
}

// Facts is project.canon and project.local.canon read and checked as every call reads them, the
// roots placed: the failures Packages reports for them (API.md O2, O3).
func (p *Project) Facts() (*Facts, error) {
	s := &snapshot{p: p, set: &source.FileSet{}, bags: map[string]*diag.Bag{}}
	s.own = diag.NewBag(s.set, "")
	if err := p.readOwn(s); err != nil {
		return nil, err
	}
	out := &Facts{Project: s.proj, Roots: make([]PlacedRoot, 0, len(s.proj.Roots))}
	for _, root := range s.proj.Roots {
		out.Roots = append(out.Roots, PlacedRoot{Name: root.Name, Dir: s.layout.RootDir(root.Name), Absent: s.layout.Absent(root.Name)})
	}
	return out, nil
}
