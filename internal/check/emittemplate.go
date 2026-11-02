package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// OutText is one out entry: the path it names, a root name's expansion included, "" for one check refuses by its form; At is its node (CODEGEN.md §2.1).
type OutText struct {
	Text string
	At   syntax.Expr
}

// OutTexts are the entries of value, the out of an emit of target in package pkg: its one entry, or each element of a list but for view; the one reader of out stage E and build share with check (CODEGEN.md §2.1, DECISIONS 229, 439).
func OutTexts(p *project.Project, pkg, target string, value syntax.Expr) []OutText {
	list, isList := value.(*syntax.ListLit)
	if !isList || target == TargetView {
		return []OutText{{Text: entryText(p, pkg, target, value), At: value}}
	}
	out := make([]OutText, len(list.Elems))
	for i, x := range list.Elems {
		out[i] = OutText{Text: entryText(p, pkg, target, x), At: x}
	}
	return out
}

// entryText is the path out entry x names, a constant string or a root name's expansion; "" for another value, an interpolation, a name with no template or a quoted root name with one.
func entryText(p *project.Project, pkg, target string, x syntax.Expr) string {
	if id, isName := x.(*syntax.IdentExpr); isName {
		text, _ := expandOut(p, pkg, target, id.Name)
		return text
	}
	s, isString := x.(syntax.StrLit)
	if !isString || !interpolationFree(x) || quotedRoot(p, pkg, target, constText(s)) {
		return ""
	}
	return constText(s)
}

// expandOut is @root/ and root's template for target expanded for pkg; false for none, or no project (CODEGEN.md §2.1).
func expandOut(p *project.Project, pkg, target, root string) (string, bool) {
	if p == nil {
		return "", false
	}
	return p.Expand(target, root, pkg)
}

// quotedRoot reports a string out equal to the name of a root with a template for target (CODEGEN.md §2.1).
func quotedRoot(p *project.Project, pkg, target, text string) bool {
	_, has := expandOut(p, pkg, target, text)
	return has
}

// outText is out entry x's path as entryText reads it, else E8009 `kind` (expected), E1132, `outTemplate` at a name never resolved in scope, or `outQuoted` (CODEGEN.md §2.1).
func (c *checker) outText(env *env, x syntax.Expr, target string, expected diag.Kind) (string, bool) {
	if id, isName := x.(*syntax.IdentExpr); isName {
		text, ok := expandOut(c.proj, env.pkg.path, target, id.Name)
		if !ok {
			c.report(env, diag.E8009.AtOutTemplate(env.span(id), id.Name, target))
		}
		return text, ok
	}
	text, ok := c.constOption(env, x, OptOut, target, expected)
	if ok && quotedRoot(c.proj, env.pkg.path, target, text) {
		c.report(env, diag.E8009.AtOutQuoted(env.span(x), text, target))
		return text, false
	}
	return text, ok
}
