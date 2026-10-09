package check

import (
	"path"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// emitConsumer is E8009 `outConsumer` at each `text` out entry under a consumer root (CODEGEN.md §2.4).
func (c *checker) emitConsumer(env *env, target string, out outOption) {
	l := c.declaredLayout(env.pkg)
	if target != TargetText || l == nil {
		return
	}
	for _, en := range out.entries {
		at, ok := l.Resolve(en.text, fileDir(env.file), source.Span{}, diag.NewBag(indexFiles(env.pkg.files), env.pkg.path))
		if !ok {
			continue
		}
		if root := ConsumerRoot(c.proj, at.Abs); root != "" {
			c.report(env, diag.E8009.AtOutConsumer(env.span(en.at), target, at.Display, root))
		}
	}
}

// ConsumerRoot is the closest consumer root holding dir, as closestRoot judges; "" for none.
func ConsumerRoot(p *project.Project, dir string) string {
	return closestRoot(p, dir, func(r project.Root) bool { return r.Consumer })
}

// closestRoot is the root keep accepts whose directory is dir or its closest ancestor,
// lexically, the first declared of two alike; "" for none.
func closestRoot(p *project.Project, dir string, keep func(project.Root) bool) string {
	best, bestLen, bestAt := "", -1, source.Pos(0)
	for _, r := range p.Roots {
		rootDir := path.Clean(r.Path)
		if _, under := Within(dir, rootDir); !under || !keep(r) {
			continue
		}
		if len(rootDir) > bestLen || len(rootDir) == bestLen && r.Span.Start < bestAt {
			best, bestLen, bestAt = r.Name, len(rootDir), r.Span.Start
		}
	}
	return best
}
