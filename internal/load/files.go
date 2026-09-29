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
	l := &Loader{FS: fsys, Layout: layout}
	c, ok := parseCall(e)
	if !ok {
		return nil
	}
	req := Request{From: from, Bag: bag}
	switch {
	case e.Method == nil:
		return l.singleJSON(c, req)
	case e.Method.Name == methodDir:
		return l.dirJSON(c, req)
	}
	return nil
}

// singleJSON is the one file of a plain load, when its format is JSON: `format:` wins over the extension.
func (l *Loader) singleJSON(c parsedCall, req Request) []File {
	p, ok := l.resolvePath(c.path, req)
	if !ok {
		return nil
	}
	format, ok := resolveCallFormat(c, p.Display, req)
	if !ok || format != types.FormatJSON || !checkOptions(loadForm, c, format, req) || !l.statFile(p, req) {
		return nil
	}
	real, ok := l.inside(p.Abs)
	if !ok {
		return nil
	}
	return []File{{Display: p.Display, Abs: real}}
}

// dirJSON is the files of a load.dir whose files the loader reads as JSON.
func (l *Loader) dirJSON(c parsedCall, req Request) []File {
	forcedJSON, err := dirForcedJSON(c)
	if err != nil || !checkOptions(methodDir, c, types.FormatJSON, req) {
		return nil
	}
	matches, ok := l.match(c.path, req)
	if !ok {
		return nil
	}
	if ok, err := l.checkFormats(matches, forcedJSON, req); err != nil || !ok {
		return nil
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
