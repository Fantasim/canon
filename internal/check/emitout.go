package check

import (
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// outEntry is one path of an emit's out, its text as written and its node.
type outEntry struct {
	text string
	at   syntax.Expr
}

// outOption is an emit's out: the entries check accepts, and the list literal when out is one (CODEGEN.md §2.1).
type outOption struct {
	entries []outEntry
	list    *syntax.ListLit
}

// emitOut is `out`, a constant string or, but for view, a non-empty list of them, each entry checked as an out (CODEGEN.md §2.1, DECISIONS 229, 269).
func (c *checker) emitOut(env *env, fi *syntax.FieldItem, target string) outOption {
	list, isList := fi.Value.(*syntax.ListLit)
	if !isList || target == TargetView {
		expected := diag.KindOutPaths
		if target == TargetView {
			expected = diag.KindConstantString
		}
		if text, ok := c.outPath(env, fi.Value, target, expected); ok {
			return outOption{entries: []outEntry{{text: text, at: fi.Value}}}
		}
		return outOption{}
	}
	if len(list.Elems) == 0 {
		c.report(env, diag.E8009.AtOutEmpty(env.span(list), target))
	}
	out := outOption{list: list}
	for _, x := range list.Elems {
		if text, ok := c.outPath(env, x, target, diag.KindOutPaths); ok {
			out.entries = append(out.entries, outEntry{text: text, at: x})
		}
	}
	return out
}

// outPath is one out path x: a constant string of portable characters (E8009 `outChars`), ending in .ts for a ts emit (E8009 `tsOut`); a path with other characters is not an entry.
func (c *checker) outPath(env *env, x syntax.Expr, target string, expected diag.Kind) (string, bool) {
	text, ok := c.constOption(env, x, OptOut, target, expected)
	if !ok {
		return text, false
	}
	if target == TargetTS && !strings.HasSuffix(text, tsSuffix) {
		c.report(env, diag.E8009.AtTsOut(env.span(x), text))
	}
	if resolved, judged := c.judgedOut(env, text); judged && !portableOut(resolved) {
		c.report(env, diag.E8009.AtOutChars(env.span(x), text, resolved, target))
		return text, false
	}
	return text, true
}

// judgedOut is the project-relative path out resolves to, which outChars judges and shows; without a layout, out as written after its @root/ prefix; false when out does not resolve, the resolver's finding alone (DECISIONS 215, 424).
func (c *checker) judgedOut(env *env, out string) (string, bool) {
	if c.declaredLayout(env.pkg) == nil {
		if after, rooted := strings.CutPrefix(out, rootSigil); rooted {
			_, out, _ = strings.Cut(after, slash)
		}
		return out, true
	}
	at, ok := c.resolveOut(env, out, source.Span{}, scratchBag(env))
	return at.Abs, ok
}

// portableOut reports a resolved path whose segments use only outCharset (CODEGEN.md §2.1, DECISIONS 424).
func portableOut(resolved string) bool {
	return strings.Trim(resolved, outCharset) == ""
}

// emitRoots is E8009 `outRoot` at the later of two entries owned by one root; an entry that does not resolve has its own finding (CODEGEN.md §2.8).
func (c *checker) emitRoots(env *env, target string, out outOption) {
	if len(out.entries) < sharingEntries || c.declaredLayout(env.pkg) == nil {
		return
	}
	first := map[string]string{} // owning root → display path of its first entry
	for _, en := range out.entries {
		at, ok := c.resolveOut(env, en.text, source.Span{}, scratchBag(env))
		if !ok {
			continue
		}
		owner := OwningRoot(c.proj, outputDir(at.Abs, en.text, target))
		if prev, shared := first[owner]; shared {
			c.report(env, diag.E8009.AtOutRoot(env.span(en.at), prev, at.Display, RootLabel(c.proj, owner), target))
			continue
		}
		first[owner] = at.Display
	}
}

// outputDir is the directory of an output resolved to abs, written as out: abs, or its parent for a ts or json file (CODEGEN.md §2.8).
func outputDir(abs, out, target string) string {
	if target == TargetTS || target == TargetView || target == TargetJSON && JSONFile(out) {
		return path.Dir(abs)
	}
	return abs
}

// OwningRoot is the root owning an output in dir, project-relative: dir's closest root, lexically, the first declared of two alike; "" for the project (CODEGEN.md §2.8, DECISIONS 269).
func OwningRoot(p *project.Project, dir string) string {
	return closestRoot(p, dir, func(project.Root) bool { return true })
}

// RootLabel is an owning root as findings name it: its name, or `project <name>` for the project (DECISIONS 269).
func RootLabel(p *project.Project, root string) string {
	if root != "" {
		return root
	}
	return projectWord + spaceSep + p.Name
}

// Within is dir relative to root when root is dir or one of its ancestors, both cleaned and project-relative, compared lexically (CODEGEN.md §2.8).
func Within(dir, root string) (string, bool) {
	switch {
	case dir == root:
		return "", true
	case root == dot:
		return dir, notAbove(dir)
	}
	rel, ok := strings.CutPrefix(dir, root+slash)
	return rel, ok && notAbove(rel)
}

// notAbove reports a relative path that does not walk back out of its root through "..".
func notAbove(rel string) bool {
	return rel != parentDir && !strings.HasPrefix(rel, parentDir+slash)
}
