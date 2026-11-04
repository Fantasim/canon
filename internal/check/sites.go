package check

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// carry is env for part, an element, entry or branch checked as part of whole (API.md F12).
func (env *env) carry(whole, part syntax.Expr) *env {
	if env.site == nil || env.siteAt != inner(whole) {
		return env
	}
	return env.storingAt(env.site, part)
}

// declSite is a declaration a stored value names: its name (none for a fn's result) and written type.
type declSite struct {
	file *syntax.File
	name *syntax.Ident
	typ  syntax.Type
}

// storingAt is env for the value v, checked against the type s declares.
func (env *env) storingAt(s *declSite, v syntax.Expr) *env {
	e := env.with()
	e.site, e.siteAt = s, inner(v)
	return e
}

// varSite is the declaration of the var o an assignment stores into, as evaluation names it.
func varSite(o *object) *declSite {
	if d, ok := o.decl.(*syntax.VarStmt); ok {
		return &declSite{file: o.file, name: d.Name, typ: d.Type}
	}
	return nil
}

// keyedBy is env for a key k checked against the key field f; none for a ref key (DECISIONS 456).
func (c *checker) keyedBy(env *env, k syntax.Expr, f *types.Field) *env {
	if f == nil || c.fieldType(f).Base().Kind() == types.Ref {
		return env
	}
	return env.storing(c.fieldObjects[f], k)
}
