package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// defineCheck is one package's walk over its defines: the first origin of each name, and the declarations already reported.
type defineCheck struct {
	s        *stage
	u        *unit
	consts   map[*Const]*constSite
	first    map[string]string
	reported map[any]bool
}

// checkCppDefines is the rules of @cpp(defines:) on a package with a cpp emit (CODEGEN.md §7.9, DECISIONS 389, 390): E8012 `cppDefine` for a constant no #define holds; E8011 once per declaration, at its first bad name; E8005 for a name a line before wrote, at the later declaration. Every mode gen/cpp writes writes the defines, so a mode check refused keeps them judged (decision 213); an unbuilt one does not.
func (s *stage) checkCppDefines(u *unit) {
	es := emitFor(u, TargetCpp)
	if es == nil || !cppPlanned(es.e) && es.e.Mode != ModeNone {
		return
	}
	c := &defineCheck{s: s, u: u, consts: map[*Const]*constSite{}, first: map[string]string{}, reported: map[any]bool{}}
	for _, site := range u.consts {
		c.consts[site.c] = site
	}
	for _, d := range CppDefineLines(u.p) {
		c.line(d)
	}
}

// line judges one define: its constant's type, then its name, then its duplicate.
func (c *defineCheck) line(d CppDefine) {
	if d.Const != nil && !slices.Contains(defineKinds, d.Const.Type.Kind) {
		site := c.consts[d.Const]
		c.u.report(diag.E8012.AtCppDefine(site.span(), d.Const.Name, site.obj.Type()))
		return
	}
	if report := defineProblem(d.Name); report != nil {
		if !c.reported[d.decl()] {
			c.reported[d.decl()] = true
			span := c.s.itemSpan(d.Member, c.s.itemSpan(d.decl(), source.Span{}))
			c.u.report(report(span, d.Name, d.origin()))
		}
		return
	}
	if prev, dup := c.first[d.Name]; dup {
		c.u.report(diag.E8005.AtDefine(c.s.itemSpan(d.decl(), source.Span{}), d.Name, prev, d.origin()))
		return
	}
	c.first[d.Name] = d.origin()
}

// defineProblem is the E8011 a define name earns, nil for a name a #define can hold: an identifier that is no keyword, no reserved name, no name the runtime headers use (they cannot be wrapped, CODEGEN.md §2.3) and no macro of the standard library; the `_`-uppercase and `__` shapes stay allowed, a define names a legacy macro (CODEGEN.md §7.9, DECISIONS 389).
func defineProblem(name string) func(source.Span, string, string) *diag.Builder {
	switch {
	case !identPattern.MatchString(name):
		return func(span source.Span, n, origin string) *diag.Builder {
			return diag.E8011.AtDerived(span, n, origin, check.TargetCpp)
		}
	case cppStdMacros[name]:
		return func(span source.Span, n, origin string) *diag.Builder { return diag.E8011.AtMacro(span, n, origin) }
	case CppReserved(name) || cppRuntimeNames()[name]:
		return func(span source.Span, n, origin string) *diag.Builder { return diag.E8011.AtReserved(span, n, origin) }
	}
	return nil
}

// targetAnnotations are the annotations that configure generated code; on a local declaration, which nothing generates, each is E1118 (GRAMMAR.md §8.3, DECISIONS 443).
var targetAnnotations = []string{syntax.AnnGo, syntax.AnnTS, syntax.AnnCpp}

// checkLocalAnnotations is E1118 for a @go, @ts or @cpp annotation on a local declaration or inside one, which no generator writes (GRAMMAR.md §8.3, DECISIONS 390, 443).
func (s *stage) checkLocalAnnotations(u *unit) {
	for _, obj := range u.cp.Decls {
		if s.info.Broken[obj] || !isLocalDecl(obj.Decl()) {
			continue
		}
		syntax.Inspect(obj.Decl(), func(n syntax.Node) bool {
			a, ok := n.(*syntax.Annotation)
			if ok && a.Name != nil && slices.Contains(targetAnnotations, a.Name.Name) {
				u.report(diag.E1118.At(obj.File().Span(a), a.Name.Name, diag.KindLocal))
			}
			return !ok
		})
	}
}
