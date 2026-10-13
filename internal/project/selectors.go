package project

import (
	"cmp"
	"path"
	"slices"
	"strings"
)

// Listing is what a selector reads of a package: its name, its directory and its file paths.
type Listing struct {
	Name   string
	Dir    string
	Files  []string
	Loose  []string // package-less files of its directory, selected by their path too (DECISIONS 353)
	Orphan bool     // files with no package of their own, counting as none (DECISIONS 353)
}

// All is every file path the listing's package or directory owns, loose ones included.
func (l Listing) All() []string {
	return slices.Concat(l.Files, l.Loose)
}

// Listings are the selectable packages of units, then those of orphan directories.
func Listings(units []*Unit, orphans []Orphan) []Listing {
	out := make([]Listing, 0, len(units)+len(orphans))
	for _, u := range units {
		l := Listing{Name: u.Name, Dir: u.Dir, Loose: u.Loose}
		for _, f := range u.Files {
			l.Files = append(l.Files, f.Src.Path)
		}
		out = append(out, l)
	}
	for _, o := range orphans {
		out = append(out, Listing{Name: o.Name, Dir: o.Dir, Files: o.Files, Orphan: true})
	}
	return out
}

// Select resolves the selectors of CLI.md §2.2 in name order, orphan directories as packages (DECISIONS 353).
func Select(units []*Unit, orphans []Orphan, selectors []string) ([]*Unit, []Orphan, error) {
	if len(selectors) == 0 {
		return slices.Clone(units), slices.Clone(orphans), nil
	}
	listings := Listings(units, orphans)
	var outUnits []*Unit
	var outOrphans []Orphan
	for _, sel := range selectors {
		matched, err := Match(listings, sel)
		if err != nil {
			return nil, nil, err
		}
		for _, i := range matched {
			if i < len(units) {
				outUnits = append(outUnits, units[i])
			} else {
				outOrphans = append(outOrphans, orphans[i-len(units)])
			}
		}
	}
	slices.SortFunc(outUnits, func(a, b *Unit) int { return cmp.Compare(a.Name, b.Name) })
	slices.SortFunc(outOrphans, func(a, b Orphan) int { return cmp.Compare(a.Name, b.Name) })
	return slices.CompactFunc(outUnits, func(a, b *Unit) bool { return a == b }),
		slices.CompactFunc(outOrphans, func(a, b Orphan) bool { return a.Name == b.Name }), nil
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
		matched = indexes(pkgs, func(l Listing) bool { return slices.Contains(l.All(), want) })
	case strings.HasSuffix(sel, allBelow):
		prefix := strings.TrimSuffix(sel, allBelow)
		matched = indexes(pkgs, func(l Listing) bool {
			return prefix != "" && (l.Name == prefix || strings.HasPrefix(l.Name, prefix+nameSep))
		})
	default:
		matched = indexes(pkgs, func(l Listing) bool { return l.Name == sel })
	}
	if len(matched) == 0 {
		return nil, &UnknownError{Err: ErrUnknownPackage, Name: sel}
	}
	return matched, nil
}

// directory is the package the files directly in the directory declare, when they agree, with the
// orphan files of the directory (DECISIONS 353); a directory without such files is the package
// named after it, if any.
func directory(pkgs []Listing, sel string) ([]int, error) {
	dir := path.Clean(sel)
	holds := func(l Listing) bool {
		return slices.ContainsFunc(l.Files, func(f string) bool { return path.Dir(f) == dir })
	}
	declared := indexes(pkgs, func(l Listing) bool { return !l.Orphan && holds(l) })
	switch len(declared) {
	case 0:
		declared = indexes(pkgs, func(l Listing) bool { return l.Dir == dir })
	case 1:
	default:
		return nil, &UnknownError{Err: ErrMixedDirectory, Name: sel}
	}
	declared = append(declared, indexes(pkgs, func(l Listing) bool { return l.Orphan && holds(l) })...)
	slices.Sort(declared)
	declared = slices.Compact(declared)
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
