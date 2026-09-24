package check

import (
	"regexp"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// emitSpec is the built-in schema of one emit target (CODEGEN.md §2.1, WIRE.md §8.1).
type emitSpec struct {
	options []string
	modes   []string
}

// checkEmits checks every `emit` of p against its target's schema, in phase 2: one emit per
// target (E8002), known targets and options (E8003), valid values (E8009, E8150). Nothing in
// an emit is an expression or is resolved in scope.
func (c *checker) checkEmits(p *pkgState) {
	seen := map[string]bool{}
	for _, f := range p.files {
		for _, d := range f.Decls {
			if e, ok := d.(*syntax.EmitDecl); ok {
				c.checkEmit(c.fileEnv(p, f, nil), e, seen)
			}
		}
	}
}

func (c *checker) checkEmit(env *env, e *syntax.EmitDecl, seen map[string]bool) {
	target := e.Target.Name
	spec, ok := emitSpecs[target]
	if !ok {
		c.report(env, diag.E8003.AtTarget(env.span(e.Target), target))
		return
	}
	if seen[target] {
		c.report(env, diag.E8002.At(env.span(e.Target), env.pkg.path, target))
	}
	seen[target] = true
	values := -1
	out := ""
	for _, it := range e.Options.Items {
		fi, isField := it.(*syntax.FieldItem)
		if !isField || !slices.Contains(spec.options, fi.Name.Name) {
			c.report(env, diag.E8003.AtOption(env.span(it), itemName(it), target))
			continue
		}
		switch fi.Name.Name {
		case optionMode:
			c.emitMode(env, fi, target, spec.modes)
		case optionValues:
			values = c.emitValues(env, fi, target)
		default:
			s, isStr := c.emitString(env, fi, target)
			if isStr && fi.Name.Name == optionOut {
				out = s
			}
		}
	}
	c.emitFileMode(env, e, target, out, values)
}

// itemName is the name of an emit option item as written.
func itemName(it syntax.BraceItem) string {
	if fi, ok := it.(*syntax.FieldItem); ok {
		return fi.Name.Name
	}
	return ""
}

// emitString is out, package or namespace: a constant string of a valid form (E8009).
func (c *checker) emitString(env *env, fi *syntax.FieldItem, target string) (string, bool) {
	s, ok := fi.Value.(syntax.StrLit)
	if !ok {
		c.report(env, diag.E8009.AtKind(env.span(fi.Value), fi.Name.Name, target, diag.KindConstantString))
		return "", false
	}
	if !interpolationFree(fi.Value) {
		c.report(env, diag.E1132.At(env.span(fi.Value)))
		return "", false
	}
	text := constText(s)
	switch {
	case fi.Name.Name == optionPackage && (!identRe.MatchString(text) || goKeywords[text]):
		c.report(env, diag.E8009.AtPackage(env.span(fi.Value), text))
	case fi.Name.Name == optionNamespace && !cppNamespace(text):
		c.report(env, diag.E8009.AtNamespace(env.span(fi.Value), text))
	case fi.Name.Name == optionOut && target == targetTS && !strings.HasSuffix(text, tsSuffix):
		c.report(env, diag.E8009.AtTsOut(env.span(fi.Value), text))
	}
	return text, true
}

var identRe = regexp.MustCompile(identPattern)

// cppNamespace reports `ident{::ident}` with no C++ keyword.
func cppNamespace(s string) bool {
	for part := range strings.SplitSeq(s, cppScope) {
		if !identRe.MatchString(part) || cppKeywords[part] {
			return false
		}
	}
	return true
}

// emitMode is `mode: word`, a mode of the target (E8009), never looked up in scope.
func (c *checker) emitMode(env *env, fi *syntax.FieldItem, target string, modes []string) {
	id, ok := fi.Value.(*syntax.IdentExpr)
	if !ok {
		c.report(env, diag.E8009.AtKind(env.span(fi.Value), fi.Name.Name, target, diag.KindModeName))
		return
	}
	if !slices.Contains(modes, id.Name) {
		c.report(env, diag.E8009.AtMode(env.span(id), id.Name, target, modes))
	}
}

// emitValues is `values: [v, …]`: public top-level lets of the package, each once (E8009); it
// returns their number.
func (c *checker) emitValues(env *env, fi *syntax.FieldItem, target string) int {
	list, ok := fi.Value.(*syntax.ListLit)
	if !ok {
		c.report(env, diag.E8009.AtKind(env.span(fi.Value), fi.Name.Name, target, diag.KindValueNames))
		return -1
	}
	seen := map[string]bool{}
	for _, x := range list.Elems {
		id, isName := x.(*syntax.IdentExpr)
		if !isName {
			c.report(env, diag.E8009.AtKind(env.span(x), fi.Name.Name, target, diag.KindValueNames))
			continue
		}
		o := env.pkg.names[id.Name]
		switch {
		case o == nil || o.kind != ObjLet || o.local:
			c.report(env, diag.E8009.AtValues(env.span(id), id.Name, target))
		case seen[id.Name]:
			c.report(env, diag.E8009.AtValuesTwice(env.span(id), id.Name, target))
		}
		seen[id.Name] = true
	}
	return len(list.Elems)
}

// emitFileMode is E8150: `emit json` whose out names a .json file writes exactly one value.
func (c *checker) emitFileMode(env *env, e *syntax.EmitDecl, target, out string, values int) {
	if target != targetJSON || !strings.HasSuffix(out, dot+targetJSON) {
		return
	}
	if values < 0 {
		values = c.publicLets(env.pkg)
	}
	if values != 1 {
		c.report(env, diag.E8150.At(env.span(e.Target), out, int64(values)))
	}
}

// publicLets counts the public top-level lets of p, the default values of `emit json`.
func (c *checker) publicLets(p *pkgState) int {
	n := 0
	for _, o := range p.all {
		if o.kind == ObjLet && !o.local {
			n++
		}
	}
	return n
}
