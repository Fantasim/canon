package load

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Named is what one load names from its literal path, by the loader's own resolution and walk:
// each file it reads, and for load.dir every directory its walk lists and every name whose links
// it resolves (API.md S3, S5, E17).
type Named struct {
	Files []File
	Dirs  []string
	Links []string
}

// Names is what the load e, written in a file of the directory from, names through fsys; false for
// a path that is no string literal (E7008), none for one that does not resolve. Its findings are
// thrown away: the load reports them when it runs.
func Names(fsys project.FS, layout *project.Layout, from string, e *syntax.LoadExpr) (Named, bool) {
	path, ok := literalPath(e)
	if !ok {
		return Named{}, false
	}
	l := &Loader{FS: fsys, Layout: layout}
	req := Request{From: from, Bag: diag.NewBag(nil, "")}
	form := loadForm
	if e.Method != nil {
		form = e.Method.Name
	}
	switch form {
	case methodDir:
		return l.dirNames(path, req), true
	case loadForm, methodCSV, methodText, formDefines:
		p, ok := l.resolvePath(path, req)
		if !ok {
			return Named{}, true
		}
		return Named{Files: []File{{Display: p.Display, Abs: p.Abs}}}, true
	default:
		return Named{}, true
	}
}

// dirNames is what load.dir(pattern) names: match's own matches, and what its walk listed and
// resolved, kept as a recorded load keeps them (memo.go), so the two never diverge.
func (l *Loader) dirNames(pattern string, req Request) Named {
	in := &Inputs{}
	l.rec = in
	matches, _ := l.match(pattern, req)
	l.rec = nil
	out := Named{Files: make([]File, len(matches))}
	for i, m := range matches {
		out.Files[i] = File(m)
	}
	if base, rest, ok := l.globBase(pattern, req.From, req.Span, req.Bag); ok && rest != "" {
		out.Dirs = append(out.Dirs, base.Abs) // its base directory, even one that does not exist yet (API.md E17)
	}
	for _, c := range in.calls {
		switch c.kind {
		case callList:
			out.Dirs = append(out.Dirs, c.name)
		case callLink:
			out.Links = append(out.Links, c.name)
		case callStat, callSource:
		}
	}
	return out
}
