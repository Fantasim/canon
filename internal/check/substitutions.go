package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// packageSubject checks `package { name: value, … }` (EVALUATION.md §10.5, DECISIONS 398).
func (c *checker) packageSubject(env *env, s *syntax.PackageSubject) {
	first := map[*object]source.Span{}
	for _, it := range s.Items {
		o := c.substitutionTarget(env, it.Name, first)
		if o == nil {
			c.expr(env, it.Value, types.ErrorType)
			continue
		}
		c.expr(env, it.Value, c.letType(o))
	}
}

// substitutionTarget resolves one target as amendBlock resolves a layer target: the let, with its
// use and dependency recorded, or nil after E5006 (not a let, another package's, given twice) or
// E2102 (unknown).
func (c *checker) substitutionTarget(env *env, id *syntax.Ident, first map[*object]source.Span) *object {
	name := id.Name
	o := c.lookupGlobal(env, name)
	if o == nil {
		o = env.lookupLocal(name)
	}
	if o == nil {
		o = c.builtins[name]
	}
	switch {
	case o == nil:
		c.unknownHinted(env, id, name, c.nearestLet(env, name))
		return nil
	case o.kind != ObjLet:
		c.report(env, diag.E5006.AtNotLet(env.span(id), name))
		return nil
	case o.pkg != env.pkg.path:
		c.report(env, diag.E5006.AtOtherPackage(env.span(id), name))
		return nil
	}
	if at, dup := first[o]; dup {
		c.report(env, diag.E5006.AtTwice(env.span(id), name, at))
		return nil
	}
	first[o] = env.span(id)
	c.useLet(env, id, o)
	return o
}

// useLet records a layer or substitution target: its use and the dependency of the owner on it.
func (c *checker) useLet(env *env, id *syntax.Ident, o *object) {
	c.info.NameUses[id] = o
	c.dependsOn(env, o)
}

// nearestLet is the top-level let of the test's package nearest to name, for an E2102 hint.
func (c *checker) nearestLet(env *env, name string) string {
	var lets []string
	for n, o := range env.pkg.names { //canon:unordered nearest sorts its candidates
		if o.kind == ObjLet {
			lets = append(lets, n)
		}
	}
	return nearest(name, lets)
}
