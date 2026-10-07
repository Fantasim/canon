package canon

import (
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/workspace"
)

// loadCall is a load call and the directory of the file it is written in.
type loadCall struct {
	from string
	e    *syntax.LoadExpr
}

// callsOf is every load call written in the files of units, whether their package checks or not.
func callsOf(units *build.Units) []loadCall {
	var out []loadCall
	for _, u := range units.Units {
		for _, f := range u.Files {
			from := path.Dir(f.Src.Path)
			syntax.Inspect(f, func(n syntax.Node) bool {
				if e, ok := n.(*syntax.LoadExpr); ok {
					out = append(out, loadCall{from: from, e: e})
				}
				return true
			})
		}
	}
	return out
}

// globs is every file the load calls of units read in s now, absolute, in byte order, whatever
// their format, by load's own matcher, walk, bounds and link rules (API.md W12); none when
// project.canon does not lay out its roots.
func (w *watching) globs(s *workspace.Snapshot, units *build.Units) []string {
	if units == nil {
		return nil
	}
	layout, ok := s.Build().Layout()
	if !ok {
		return nil
	}
	bag := diag.NewBag(&source.FileSet{}, "")
	var out []string
	for _, c := range callsOf(units) {
		for _, f := range load.Files(s.Build().FS(), layout, c.from, c.e, bag) {
			out = append(out, f.Abs)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// holds reports name a file of matched, a sorted list, or, unless its own listing changed, a
// directory holding one: a new directory with a matching file in it.
func holds(matched []string, name string, relisted bool) bool {
	if _, found := slices.BinarySearch(matched, name); found {
		return true
	}
	return !relisted && slices.ContainsFunc(matched, func(f string) bool { return below(f, []string{name}) })
}
