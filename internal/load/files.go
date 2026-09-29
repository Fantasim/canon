package load

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// File is a file a load call reads: its display path and its resolved path, a link followed.
type File struct {
	Display string
	Abs     string
}

// JSONFiles is the JSON files of a literal load call e, written in the directory from, that lie in the project or a declared root; none for a load.dir whose formats the loader refuses (WIRE.md §6.5).
func JSONFiles(fsys project.FS, layout *project.Layout, from string, e *syntax.LoadExpr, bag *diag.Bag) []File {
	return (&Loader{FS: fsys, Layout: layout}).files(from, e, bag, true)
}

// Files is every file a literal load call e, written in the directory from, reads, whatever its format, by the loader's own matcher, walk, bounds and link rules (WIRE.md §6.1, §6.5).
func Files(fsys project.FS, layout *project.Layout, from string, e *syntax.LoadExpr, bag *diag.Bag) []File {
	return (&Loader{FS: fsys, Layout: layout}).files(from, e, bag, false)
}

// files is Files, or with jsonOnly JSONFiles: the files e reads as JSON.
func (l *Loader) files(from string, e *syntax.LoadExpr, bag *diag.Bag, jsonOnly bool) []File {
	c, ok := parseCall(e)
	if !ok {
		return nil
	}
	req := Request{From: from, Bag: bag}
	switch {
	case e.Method == nil:
		return l.singleFile(c, req, jsonOnly)
	case e.Method.Name == methodDir:
		return l.dirFiles(c, req, jsonOnly)
	}
	return nil
}

// singleFile is the one file of a plain load; with jsonOnly, when its format is JSON: `format:` wins over the extension.
func (l *Loader) singleFile(c parsedCall, req Request, jsonOnly bool) []File {
	p, ok := l.resolvePath(c.path, req)
	if !ok {
		return nil
	}
	format, ok := resolveCallFormat(c, p.Display, req)
	if !ok || (jsonOnly && format != types.FormatJSON) || !checkOptions(loadForm, c, format, req) || !l.statFile(p, req) {
		return nil
	}
	real, ok := l.inside(p.Abs)
	if !ok {
		return nil
	}
	return []File{{Display: p.Display, Abs: real}}
}

// dirFiles is the files a load.dir matches; with jsonOnly, only when the loader reads them all as JSON.
func (l *Loader) dirFiles(c parsedCall, req Request, jsonOnly bool) []File {
	forcedJSON, err := dirForcedJSON(c)
	format := types.FormatJSON
	if !jsonOnly {
		format = dirFormat(c)
	}
	if (jsonOnly && err != nil) || !checkOptions(methodDir, c, format, req) {
		return nil
	}
	matches, ok := l.match(c.path, req)
	if !ok {
		return nil
	}
	if jsonOnly {
		if ok, err := l.checkFormats(matches, forcedJSON, req); err != nil || !ok {
			return nil
		}
	}
	out := make([]File, len(matches))
	for i, m := range matches {
		out[i] = File(m)
	}
	return out
}

// Inside is the resolved path of abs when it lies in the project or a declared root, false for any link that leaves them or does not resolve (WIRE.md §6.5).
func Inside(fsys project.FS, layout *project.Layout, abs string) (string, bool) {
	return (&Loader{FS: fsys, Layout: layout}).inside(abs)
}

func (l *Loader) inside(abs string) (string, bool) {
	if _, ok := l.FS.(interface{ EvalSymlinks(string) (string, error) }); !ok {
		return abs, true // a file system without links: abs is its own real path
	}
	real, err := project.EvalSymlinks(l.FS, abs)
	if err != nil || (real != abs && !within(real, l.walkBounds())) {
		return "", false
	}
	return real, true
}

// dirFormat is a load.dir call's own format, which its options are checked against: `format:`,
// else its pattern's extension, else JSON's, as the options table has it.
func dirFormat(c parsedCall) types.LoadFormat {
	if c.format != nil {
		return types.FormatNamed(*c.format)
	}
	if f := types.FormatOfPath(c.path); f != types.FormatUnknown {
		return f
	}
	return types.FormatJSON
}
