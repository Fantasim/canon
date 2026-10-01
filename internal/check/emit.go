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
	var out outOption
	c.emitRequired(env, e, target)
	for _, it := range e.Options.Items {
		fi, isField := it.(*syntax.FieldItem)
		if !isField || !slices.Contains(spec.options, fi.Name.Name) {
			c.report(env, diag.E8003.AtOption(env.span(it), itemName(it), target))
			continue
		}
		switch fi.Name.Name {
		case OptMode:
			c.emitMode(env, fi, target, spec.modes)
		case OptValues:
			values = c.emitValues(env, fi, target)
		case OptOut:
			out = c.emitOut(env, fi, target)
		default:
			c.emitString(env, fi, target)
		}
	}
	c.emitFileMode(env, e, target, out, values)
	c.emitRoots(env, target, out)
	switch {
	case target == TargetGo && !hasOption(e, OptPackage):
		c.defaultGoPackage(env, e, out)
	case target == TargetCpp && !hasOption(e, OptNamespace):
		c.defaultCppNamespace(env, e)
	}
}

// emitRequired is E8009 `missing`: every target's `out` is required, with no default (CODEGEN.md §2.1, WIRE.md §8.1).
func (c *checker) emitRequired(env *env, e *syntax.EmitDecl, target string) {
	if !hasOption(e, OptOut) {
		c.report(env, diag.E8009.AtMissing(env.span(e.Target), OptOut, target))
	}
}

// hasOption reports an option written in e, of any value.
func hasOption(e *syntax.EmitDecl, name string) bool {
	return slices.ContainsFunc(e.Options.Items, func(it syntax.BraceItem) bool { return itemName(it) == name })
}

// defaultGoPackage validates the last element of out, which every entry of a list shares, as the package (CODEGEN.md §2.1, DECISIONS 213, 269).
func (c *checker) defaultGoPackage(env *env, e *syntax.EmitDecl, out outOption) {
	var names []string
	for _, en := range out.entries {
		if name, known := c.lastElement(env, en.text); known {
			names = append(names, name)
		}
	}
	switch {
	case len(names) == 0:
	case slices.ContainsFunc(names, func(n string) bool { return n != names[0] }):
		c.report(env, diag.E8009.AtOutPackage(env.span(out.list)))
	case !identRe.MatchString(names[0]) || goKeywords[names[0]]:
		c.report(env, diag.E8009.AtPackage(env.span(e.Target), names[0]))
	}
}

// defaultCppNamespace checks the default namespace as a written one (CODEGEN.md §2.1; log-2026-09-25).
func (c *checker) defaultCppNamespace(env *env, e *syntax.EmitDecl) {
	c.namespaceFinding(env, e.Target, strings.ReplaceAll(env.pkg.path, dot, cppScope))
}

// namespaceFinding is E8009 at at for a namespace not `ident{::ident}`, using a C++ keyword or a reserved segment.
func (c *checker) namespaceFinding(env *env, at syntax.Node, ns string) {
	switch {
	case !cppNamespace(ns):
		c.report(env, diag.E8009.AtNamespace(env.span(at), ns))
	case reservedNamespace(ns):
		c.report(env, diag.E8009.AtReservedNamespace(env.span(at), ns))
	}
}

// itemName is the name of an emit option item as written.
func itemName(it syntax.BraceItem) string {
	if fi, ok := it.(*syntax.FieldItem); ok {
		return fi.Name.Name
	}
	return ""
}

// emitString is package or namespace: a constant string of a valid form (E8009).
func (c *checker) emitString(env *env, fi *syntax.FieldItem, target string) {
	text, ok := c.constOption(env, fi.Value, fi.Name.Name, target, diag.KindConstantString)
	switch {
	case !ok:
	case fi.Name.Name == OptPackage && (!identRe.MatchString(text) || goKeywords[text]):
		c.report(env, diag.E8009.AtPackage(env.span(fi.Value), text))
	case fi.Name.Name == OptNamespace:
		c.namespaceFinding(env, fi.Value, text)
	}
}

// constOption is x's text when the value of option is a constant string, else E8009 `kind` naming expected, or E1132 (CODEGEN.md §2.1).
func (c *checker) constOption(env *env, x syntax.Expr, option, target string, expected diag.Kind) (string, bool) {
	s, ok := x.(syntax.StrLit)
	if !ok {
		c.report(env, diag.E8009.AtKind(env.span(x), option, target, expected))
		return "", false
	}
	if c.lexError(x) {
		return "", false // its text is made up (DECISIONS 215)
	}
	if !interpolationFree(x) {
		c.report(env, diag.E1132.At(env.span(x)))
		return "", false
	}
	return constText(s), true
}

var identRe = regexp.MustCompile(identPattern)

// IsGoKeyword reports a Go keyword, which a Go package name may not be and a generated lower-case name escapes (CODEGEN.md §2.1, §3.4).
func IsGoKeyword(s string) bool { return goKeywords[s] }

// IsCppKeyword reports a C++20 keyword or alternative token, which a namespace may not use and a generated verbatim name escapes (CODEGEN.md §2.1, §3.4).
func IsCppKeyword(s string) bool { return cppKeywords[s] }

// IsCppNamespace reports canon, std or nlohmann: a namespace generated C++ names (CODEGEN.md §3.4).
func IsCppNamespace(s string) bool { return cppNamespaces[s] }

// cppNamespace reports `ident{::ident}` with no C++ keyword (CODEGEN.md §2.1).
func cppNamespace(s string) bool {
	for part := range strings.SplitSeq(s, cppScope) {
		if !identRe.MatchString(part) || cppKeywords[part] {
			return false
		}
	}
	return true
}

// reservedNamespace reports a segment naming canon, std or nlohmann, which generated C++ uses (CODEGEN.md §3.4).
func reservedNamespace(s string) bool {
	for part := range strings.SplitSeq(s, cppScope) {
		if cppNamespaces[part] {
			return true
		}
	}
	return false
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

// emitFileMode is E8150, judged once by a list's first entry, and E8009 `outForm` for a list mixing files and directories (WIRE.md §8.1).
func (c *checker) emitFileMode(env *env, e *syntax.EmitDecl, target string, out outOption, values int) {
	if target != TargetJSON || len(out.entries) == 0 {
		return
	}
	files := 0
	for _, en := range out.entries {
		if JSONFile(en.text) {
			files++
		}
	}
	switch {
	case files == 0:
		return
	case files < len(out.entries):
		c.report(env, diag.E8009.AtOutForm(env.span(out.list)))
		return
	}
	if values < 0 {
		values = c.publicLets(env.pkg)
	}
	if values != 1 {
		c.report(env, diag.E8150.At(env.span(e.Target), out.entries[0].text, int64(values)))
	}
}

// JSONFile reports an `emit json` out in file mode: it ends in .json (WIRE.md §8.1).
func JSONFile(out string) bool {
	return strings.HasSuffix(out, dot+TargetJSON)
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
