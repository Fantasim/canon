package check

import (
	"path"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// lastElement is the last element of the directory out, written in env's file, names against
// the roots as written: "" for the project directory itself, false when out does not resolve.
func (c *checker) lastElement(env *env, out string) (string, bool) {
	p := env.pkg
	l := c.declaredLayout(p)
	if l == nil || out == "" {
		return "", false
	}
	at, ok := l.Resolve(out, fileDir(env.file), source.Span{}, diag.NewBag(indexFiles(p.files), p.path))
	switch {
	case !ok:
		return "", false
	case at.Abs == l.Dir:
		return "", true
	}
	return path.Base(at.Abs), true
}

// declaredLayout places the project at layoutAnchor with its roots as written, never a --root
// override or a checkout path (DECISIONS 215).
func (c *checker) declaredLayout(p *pkgState) *project.Layout {
	if c.layout == nil && c.proj != nil {
		c.layout, _ = project.NewLayout(c.proj, layoutAnchor, nil, diag.NewBag(indexFiles(p.files), p.path))
	}
	return c.layout
}

// contained is E7001 for a path of env's file leaving its root or the project (WIRE.md §2.2 rule 3).
func (c *checker) contained(env *env, at syntax.Node, written string) bool {
	l := c.declaredLayout(env.pkg)
	if l == nil {
		return true
	}
	dir, span := fileDir(env.file), env.span(at)
	if _, ok := l.Resolve(written, dir, span, diag.NewBag(indexFiles(env.pkg.files), env.pkg.path)); ok {
		return true
	}
	c.emit(env, func(bag *diag.Bag) { l.Resolve(written, dir, span, bag) })
	c.counted(env)
	return false
}
