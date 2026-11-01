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
	at, ok := c.resolveOut(env, out, source.Span{}, scratchBag(env))
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

// resolveOut resolves out, written in env's file, against the declared layout, reporting into bag; false when it does not resolve or there is no layout (WIRE.md §2).
func (c *checker) resolveOut(env *env, out string, span source.Span, bag *diag.Bag) (project.Path, bool) {
	l := c.declaredLayout(env.pkg)
	if l == nil {
		return project.Path{}, false
	}
	return l.Resolve(out, fileDir(env.file), span, bag)
}

// scratchBag is a bag whose findings are dropped, for a resolution made only to learn whether it succeeds.
func scratchBag(env *env) *diag.Bag {
	return diag.NewBag(indexFiles(env.pkg.files), env.pkg.path)
}

// contained is E7001 for a path of env's file leaving its root or the project (WIRE.md §2.2 rule 3).
func (c *checker) contained(env *env, at syntax.Node, written string) bool {
	if c.declaredLayout(env.pkg) == nil {
		return true
	}
	span := env.span(at)
	if _, ok := c.resolveOut(env, written, span, scratchBag(env)); ok {
		return true
	}
	c.emit(env, func(bag *diag.Bag) { c.resolveOut(env, written, span, bag) })
	c.counted(env)
	return false
}
