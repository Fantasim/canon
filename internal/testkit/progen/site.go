package progen

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
)

// Edit replaces the bytes [Start, End) of a file by Text.
type Edit struct {
	Start, End int
	Text       string
}

// Site is one application of a mutation operator: edits of the file Path (new when absent), the
// focus edit whose text is where the expected finding starts, files to add, and the packages
// and layers the check runs with.
type Site struct {
	Path      string
	Edits     []Edit
	Focus     int
	Add       map[string][]byte
	Links     map[string]string // symbolic links to add, by path: their targets
	Packages  []string
	Layers    []string // the layers the check activates
	Elsewhere *Place   // where the finding is expected when it is not the focus edit's text
}

// Place is a region of a file of a mutated project.
type Place struct {
	Path string
	Region
}

// Mutated is a site applied: the project, where the expected finding starts, every region the
// site wrote (each edit's text, each added file whole), which shrinking keeps, and what each held.
type Mutated struct {
	Project *Project
	At      Place
	Written []Place
	Was     []Undo
}

// Undo is what a written region held before the site: its text, or no file at all.
type Undo struct {
	Text   []byte
	Absent bool
}

// Mutate is p with the site applied; an error for edits outside the file, overlapping, or a
// focus that names no edit.
func (s Site) Mutate(p *Project) (Mutated, error) {
	src, _ := p.Get(s.Path)
	text, regions, err := s.apply(src)
	if err != nil {
		return Mutated{}, err
	}
	out := Mutated{Project: p.Clone()}
	out.Project.Set(s.Path, text)
	for i, r := range regions {
		out.Written = append(out.Written, Place{Path: s.Path, Region: r})
		out.Was = append(out.Was, Undo{Text: src[s.Edits[i].Start:s.Edits[i].End]})
	}
	for _, name := range slices.Sorted(maps.Keys(s.Add)) {
		old, ok := p.Get(name)
		out.Project.Set(name, s.Add[name])
		out.Written = append(out.Written, Place{Path: name, Region: Region{End: len(s.Add[name])}})
		out.Was = append(out.Was, Undo{Text: old, Absent: !ok})
	}
	for _, name := range slices.Sorted(maps.Keys(s.Links)) {
		out.Project.Link(name, s.Links[name])
	}
	switch {
	case s.Elsewhere != nil:
		out.At = *s.Elsewhere
	case s.Focus < 0 || s.Focus >= len(regions):
		return Mutated{}, fmt.Errorf("%w: focus %d of %d edits", errSite, s.Focus, len(regions))
	default:
		out.At = Place{Path: s.Path, Region: regions[s.Focus]}
	}
	return out, nil
}

// apply is src with the site's edits made, and the region each edit's text occupies in the
// result, in the order of Edits.
func (s Site) apply(src []byte) ([]byte, []Region, error) {
	order := make([]int, len(s.Edits))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return s.Edits[a].Start - s.Edits[b].Start })
	var out bytes.Buffer
	regions := make([]Region, len(s.Edits))
	last := 0
	for _, i := range order {
		e := s.Edits[i]
		if e.Start < last || e.End < e.Start || e.End > len(src) {
			return nil, nil, fmt.Errorf("%w: edit [%d, %d) in %s of %d bytes", errSite, e.Start, e.End, s.Path, len(src))
		}
		out.Write(src[last:e.Start])
		regions[i] = Region{Start: out.Len(), End: out.Len() + len(e.Text)}
		out.WriteString(e.Text)
		last = e.End
	}
	out.Write(src[last:])
	return out.Bytes(), regions, nil
}
