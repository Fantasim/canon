package check

import (
	"path"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
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
