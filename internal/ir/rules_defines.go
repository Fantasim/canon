package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// checkDefineRefs is E8012 for a baked go emit: a ref into a load.defines table, which gen/go does not generate
// yet (decisions 180, 194), in a field, value, constant, export fn or dependent branch, through another package's
// types too, as the IR's Defines are gathered.
func (s *stage) checkDefineRefs(u *unit) {
	if !bakedFor(u, TargetGo) {
		return
	}
	own := u.p.Name
	s.eachOwnField(u, func(owner string, f *Field) {
		if site := s.fieldSites[f]; reachesDefine(own, &f.Type) {
			u.report(diag.E8012.At(site.span(), owner+qnameSep+f.Name, site.tf.Type))
		}
	})
	for _, v := range u.values {
		if reachesDefine(own, &v.v.Type) {
			u.report(diag.E8012.At(v.span().span(), v.v.Name, v.t))
		}
	}
	for _, c := range u.consts {
		if reachesDefine(own, &c.c.Type) {
			u.report(diag.E8012.At(c.span(), c.c.Name, c.obj.Type()))
		}
	}
	for _, site := range s.ownFns(u) {
		s.fnDefineRefs(u, site)
	}
	for _, t := range u.p.Types {
		if d, ok := t.(*Dependent); ok {
			s.branchDefineRefs(u, d)
		}
	}
}

// fnDefineRefs reports each parameter and the result of an export fn that reaches a define table.
func (s *stage) fnDefineRefs(u *unit, site *fnSite) {
	span := site.span()
	for i, p := range site.fn.Params {
		if i < len(site.sig.Params) && reachesDefine(u.p.Name, &p.Type) {
			u.report(diag.E8012.At(s.itemSpan(p, span), site.label, site.sig.Params[i]))
		}
	}
	if reachesDefine(u.p.Name, &site.fn.Result) {
		u.report(diag.E8012.At(span, site.label, site.sig.Result))
	}
}

// branchDefineRefs reports each branch of a dependent type that reaches a define table (CODEGEN.md §5.6).
func (s *stage) branchDefineRefs(u *unit, d *Dependent) {
	for _, br := range d.Branches {
		if reachesDefine(u.p.Name, &br.Type) {
			u.report(diag.E8012.At(s.itemSpan(d, source.Span{}), d.Name+qnameSep+br.Name, s.branchTypes[br]))
		}
	}
}

// reachesDefine reports a ref into a define table in t, entering another package's named types, which only their own package would judge; this package's are judged at their own fields.
func reachesDefine(own string, t *TypeRef) bool {
	found := false
	w := newWalker(func(n Type) bool { return pkgOf(n) != own }, func(r *TypeRef) {
		found = found || r.Ref != nil && r.Ref.Coll == types.CollDefines
	})
	w.ref(t)
	return found
}
