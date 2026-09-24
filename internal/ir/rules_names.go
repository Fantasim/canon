package ir

import (
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// checkOverrideNames is E8011: a @go/@cpp/@ts(name:) override that is not a valid identifier for its target, or is reserved there (CODEGEN.md §3.5, decision 120); an enum member's or a variant case's own override has no tracked span yet and is not checked here.
func (s *stage) checkOverrideNames(u *unit) {
	for _, t := range u.p.Types {
		span := s.decls[t].span()
		switch x := t.(type) {
		case *Record:
			s.checkNames(u, span, x.Go.Name, x.Cpp.Name, x.TS.Name)
			s.overrideFields(u, x.Fields)
			s.overrideFns(u, x.Methods)
		case *Enum:
			s.checkNames(u, span, x.Go.Name, x.Cpp.Name, x.TS.Name)
		case *Variant:
			s.checkNames(u, span, x.Go.Name, x.Cpp.Name, x.TS.Name)
			for _, c := range x.Cases {
				s.overrideFields(u, c.Fields)
				s.overrideFns(u, c.Methods)
			}
		case *Dependent:
			s.checkNames(u, span, x.Go.Name, x.Cpp.Name, x.TS.Name)
		}
	}
	for _, c := range u.consts {
		s.checkNames(u, c.span(), c.c.Go.Name, c.c.Cpp.Name, c.c.TS.Name)
	}
	for _, v := range u.values {
		s.checkNames(u, v.span().span(), v.v.Go.Name, v.v.Cpp.Name, v.v.TS.Name)
	}
}

func (s *stage) overrideFields(u *unit, fields []*Field) {
	for _, f := range fields {
		s.checkNames(u, s.fieldSites[f].span(), f.Go.Name, f.Cpp.Name, f.TS.Name)
	}
}

func (s *stage) overrideFns(u *unit, fns []*ExportFn) {
	for _, fn := range fns {
		if site := s.fnObjs[fn]; site != nil {
			s.checkNames(u, site.span(), fn.Go.Name, fn.Cpp.Name, fn.TS.Name)
		}
	}
}

// checkNames reports E8011 for each non-empty override that its own emitted target holds but cannot use.
func (s *stage) checkNames(u *unit, span source.Span, goName, cppName, tsName string) {
	if hasTarget(u, TargetGo) && goName != "" && !goValidIdent(goName) {
		u.report(diag.E8011.At(span, goName, check.TargetGo))
	}
	if hasTarget(u, TargetCpp) && cppName != "" && !cppValidIdent(cppName) {
		u.report(diag.E8011.At(span, cppName, check.TargetCpp))
	}
	if hasTarget(u, TargetTS) && tsName != "" && !identPattern.MatchString(tsName) {
		u.report(diag.E8011.At(span, tsName, check.TargetTS))
	}
}

// checkNameCollisions is E8005 for the go target: two generated names equal in one scope (CODEGEN.md §3.5, decision 120); cpp and ts have no backend yet to compute their own names against.
func (s *stage) checkNameCollisions(u *unit) {
	if !hasTarget(u, TargetGo) {
		return
	}
	s.checkPackageScope(u)
	for _, t := range u.p.Types {
		switch x := t.(type) {
		case *Record:
			s.checkMemberScope(u, x.QName(), x.Fields, x.Methods)
		case *Enum:
			s.checkEnumScope(u, x)
		case *Variant:
			for _, c := range x.Cases {
				s.checkMemberScope(u, x.QName()+qnameSep+c.Name, c.Fields, c.Methods)
			}
		}
	}
}

// nameScope tracks the origin of every generated name reported once in one scope, so a repeat reports E8005 naming both origins.
type nameScope struct {
	seen map[string]string
}

func (n *nameScope) add(u *unit, span source.Span, name, origin string) {
	if prev, ok := n.seen[name]; ok {
		u.report(diag.E8005.At(span, check.TargetGo, name, prev, origin))
		return
	}
	if n.seen == nil {
		n.seen = map[string]string{}
	}
	n.seen[name] = origin
}

// checkPackageScope is E8005 among a package's own type names, value containers, constants and package-level export fns (CODEGEN.md §3.3, §3.5).
func (s *stage) checkPackageScope(u *unit) {
	var scope nameScope
	for _, t := range u.p.Types {
		if name, origin, span := s.typeGoName(t); name != "" {
			scope.add(u, span, name, origin)
		}
	}
	for _, v := range u.values {
		// only a table or a keyed list gets its own container class (CODEGEN.md §5.9); a record or variant value is the type itself, naming nothing new.
		t := v.v.Type
		if t.Kind == types.Table || t.Kind == types.List && t.KeyedBy != nil {
			scope.add(u, v.span().span(), GoUpperCamel(v.v.Name), v.v.Name)
		}
	}
	for _, c := range u.consts {
		scope.add(u, c.span(), effectiveGo(c.c.Go, c.c.Name), c.c.Name)
	}
	for _, site := range u.fns {
		scope.add(u, site.span(), effectiveGo(site.fn.Go, site.fn.Name), site.label)
	}
}

// checkMemberScope is E8005 among one record's or case's own fields and methods (CODEGEN.md §3.5: "a field strong next to an export fn getStrong").
func (s *stage) checkMemberScope(u *unit, owner string, fields []*Field, fns []*ExportFn) {
	var scope nameScope
	for _, f := range fields {
		scope.add(u, s.fieldSites[f].span(), effectiveGo(f.Go, f.Name), owner+qnameSep+f.Name)
	}
	for _, fn := range fns {
		if site := s.fnObjs[fn]; site != nil {
			scope.add(u, site.span(), effectiveGo(fn.Go, fn.Name), site.label)
		}
	}
}

// checkEnumScope is E8005 among one enum's own members (CODEGEN.md §3.5: "series_1 and series1 in one enum").
func (s *stage) checkEnumScope(u *unit, e *Enum) {
	var scope nameScope
	span := s.decls[e].span()
	for _, m := range e.Members {
		scope.add(u, span, effectiveGo(m.Go, m.Name), e.QName()+qnameSep+m.Name)
	}
}

// typeGoName is a public type's own Go name package-scope: "T", first letter uppercased, or its @go(name:) override (CODEGEN.md §3.3).
func (s *stage) typeGoName(t Type) (name, origin string, span source.Span) {
	switch x := t.(type) {
	case *Record:
		return effectiveTypeName(x.Go, x.Name), x.Name, s.decls[t].span()
	case *Enum:
		return effectiveTypeName(x.Go, x.Name), x.Name, s.decls[t].span()
	case *Variant:
		return effectiveTypeName(x.Go, x.Name), x.Name, s.decls[t].span()
	case *Dependent:
		return effectiveTypeName(x.Go, x.Name), x.Name, s.decls[t].span()
	}
	return "", "", source.Span{}
}

func effectiveTypeName(n NameOptions, canon string) string {
	if n.Name != "" {
		return n.Name
	}
	if canon == "" {
		return canon
	}
	return strings.ToUpper(canon[:1]) + canon[1:]
}

// effectiveGo is a member, field, constant or export fn's Go name: its @go(name:) override, or GoUpperCamel of its Canon name.
func effectiveGo(n NameOptions, canon string) string {
	if n.Name != "" {
		return n.Name
	}
	return GoUpperCamel(canon)
}
