package project

import (
	"cmp"
	"path"
	"slices"
	"strings"
)

// Listing is what a selector reads of a package: its name, its directory and its file paths.
type Listing struct {
	Name  string
	Dir   string
	Files []string
}

// Select resolves the selectors of CLI.md §2.2 in name order; an unmatched one is ErrUnknownPackage.
func Select(units []*Unit, selectors []string) ([]*Unit, error) {
	if len(selectors) == 0 {
		return slices.Clone(units), nil
	}
	listings := make([]Listing, len(units))
	for i, u := range units {
		listings[i] = Listing{Name: u.Name, Dir: u.Dir}
		for _, f := range u.Files {
			listings[i].Files = append(listings[i].Files, f.Src.Path)
		}
	}
	var out []*Unit
	for _, sel := range selectors {
		matched, err := Match(listings, sel)
		if err != nil {
			return nil, err
		}
		for _, i := range matched {
			out = append(out, units[i])
		}
	}
	slices.SortFunc(out, func(a, b *Unit) int { return cmp.Compare(a.Name, b.Name) })
	return slices.CompactFunc(out, func(a, b *Unit) bool { return a == b }), nil
}

// Match is the indexes of the packages sel selects, or an *UnknownError: `a.b` one package,
// `a.b...` it and those below, `./a/b` a directory's, `a/b/f.canon` a file's.
func Match(pkgs []Listing, sel string) ([]int, error) {
	var matched []int
	switch {
	case strings.HasPrefix(sel, relPrefix):
		return directory(pkgs, sel)
	case path.Ext(sel) == SourceExt:
		want := path.Clean(sel)
		matched = indexes(pkgs, func(l Listing) bool { return slices.Contains(l.Files, want) })
	case strings.HasSuffix(sel, allBelow):
		prefix := strings.TrimSuffix(sel, allBelow)
		matched = indexes(pkgs, func(l Listing) bool { return l.Name == prefix || strings.HasPrefix(l.Name, prefix+nameSep) })
	default:
		matched = indexes(pkgs, func(l Listing) bool { return l.Name == sel })
	}
	if len(matched) == 0 {
		return nil, &UnknownError{Err: ErrUnknownPackage, Name: sel}
	}
	return matched, nil
}

// directory is the package the files directly in the directory declare, when they agree; a
// directory without such files is the package named after it, if any.
func directory(pkgs []Listing, sel string) ([]int, error) {
	dir := path.Clean(sel)
	declared := indexes(pkgs, func(l Listing) bool {
		return slices.ContainsFunc(l.Files, func(f string) bool { return path.Dir(f) == dir })
	})
	switch len(declared) {
	case 0:
		declared = indexes(pkgs, func(l Listing) bool { return l.Dir == dir })
	case 1:
	default:
		return nil, &UnknownError{Err: ErrMixedDirectory, Name: sel}
	}
	if len(declared) == 0 {
		return nil, &UnknownError{Err: ErrUnknownPackage, Name: sel}
	}
	return declared, nil
}

func indexes(pkgs []Listing, keep func(Listing) bool) []int {
	var out []int
	for i, l := range pkgs {
		if keep(l) {
			out = append(out, i)
		}
	}
	return out
}
