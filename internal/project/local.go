package project

import (
	"cmp"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Local is a checked project.local.canon: the declared roots it moves, in name order, each
// Path as written, absolute or relative to the project directory (DECISIONS 332).
type Local struct {
	Roots []Root
}

// LoadLocal reads project.local.canon, parsed as project.canon is, against the project p
// declares. After an error finding the result is nil and err is ErrInvalid.
func LoadLocal(src *source.File, p *Project, bag *diag.Bag) (*Local, error) {
	before := bag.Summary().Errors
	f := syntax.Parse(src, syntax.FileProject, bag)
	if f.Project == nil {
		return nil, ErrInvalid
	}
	ls := &localSchema{schema: &schema{f: f, bag: bag}, decl: p, local: &Local{}, given: map[string]bool{}}
	if name := f.Project.Name.Name; name != p.Name {
		ls.fail(diag.E1014.AtName(f.Span(f.Project.Name), name, p.Name))
	}
	for _, e := range f.Project.Items {
		ls.item(e)
	}
	if ls.failed || bag.Summary().Errors > before {
		return nil, ErrInvalid
	}
	slices.SortFunc(ls.local.Roots, func(a, b Root) int { return cmp.Compare(a.Name, b.Name) })
	return ls.local, nil
}

// localSchema reads the one key of project.local.canon, roots, into local.
type localSchema struct {
	*schema
	decl  *Project
	local *Local
	roots bool            // the roots key was given
	given map[string]bool // the root names it moves
}

// item is `roots`, once, a map (E1014 key, duplicate, roots).
func (ls *localSchema) item(e *syntax.ProjectEntry) {
	name, _ := ls.keyName(e.Key)
	span := ls.span(e.Key)
	switch {
	case isBad(e.Key):
		return
	case name != keyRoots:
		ls.fail(diag.E1014.AtKey(span, name))
		return
	case ls.roots:
		ls.fail(diag.E1014.AtDuplicate(span, name))
		return
	}
	ls.roots = true
	if isBad(e.Value) {
		return
	}
	m, ok := e.Value.(*syntax.ProjectMap)
	if !ok {
		ls.fail(diag.E1014.AtRoots(ls.span(e.Value)))
		return
	}
	for _, r := range m.Entries {
		ls.root(r)
	}
}

// root moves one declared root, given once, to a non-empty path string (E1014 root, duplicate,
// path, empty); a root's key is its text, as written.
func (ls *localSchema) root(e *syntax.ProjectEntry) {
	name, _ := ls.keyName(e.Key)
	span := ls.span(e.Key)
	_, declared := ls.decl.Root(name)
	switch {
	case isBad(e.Key):
		return
	case !declared:
		ls.fail(diag.E1014.AtRoot(span, name, rootNames(ls.decl)))
		return
	case ls.given[name]:
		ls.fail(diag.E1014.AtDuplicate(span, name))
		return
	}
	ls.given[name] = true
	if isBad(e.Value) {
		return
	}
	path, ok := stringOf(e.Value)
	switch {
	case !ok:
		ls.fail(diag.E1014.AtPath(ls.span(e.Value), name))
	case path == "":
		ls.fail(diag.E1014.AtEmpty(ls.span(e.Value), name))
	default:
		ls.local.Roots = append(ls.local.Roots, Root{Name: name, Path: path, Span: span})
	}
}

// rootNames is the names of p's roots, in order.
func rootNames(p *Project) []string {
	out := make([]string, len(p.Roots))
	for i, r := range p.Roots {
		out[i] = r.Name
	}
	return out
}
