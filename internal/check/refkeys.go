package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// refKey is a `keyed by` field of ref type, its chain of keys judged once refs resolve.
type refKey struct {
	env  *env
	at   *syntax.Ident
	name string
	typ  types.Type
}

// refKey notes a ref key field typed t, at once after its package's resolution (TYPES.md §9.1).
func (c *checker) refKey(env *env, at *syntax.Ident, t types.Type) {
	if t.Base().Kind() != types.Ref {
		return
	}
	k := refKey{env: env, at: at, name: at.Name, typ: t}
	if env.pkg.keyed {
		c.judgeRefKey(k)
		return
	}
	env.pkg.refKeys = append(env.pkg.refKeys, k)
}

// checkRefKeys judges the ref keys p's resolution met; later ones are judged when met.
func (c *checker) checkRefKeys(p *pkgState) {
	p.keyed = true
	for _, k := range p.refKeys {
		c.judgeRefKey(k)
	}
	p.refKeys = nil
}

// judgeRefKey is E3012 for a ref key whose chain of keys never reaches a base type (TYPES.md §9.1).
func (c *checker) judgeRefKey(k refKey) {
	if !c.keyLoops(k.typ) {
		return
	}
	c.report(k.env, diag.E3012.AtCycle(k.env.span(k.at), k.name, k.typ))
}

// keyLoops reports a ref key type whose targets' keys come back to a target already met.
func (c *checker) keyLoops(t types.Type) bool {
	seen := map[*types.Collection]bool{}
	for {
		r, ok := t.Base().(*types.RefType)
		if !ok || c.brokenRef(r) {
			return false // a base key, or a target in error with its own finding
		}
		coll := c.coll(r)
		if seen[coll] {
			return true
		}
		seen[coll] = true
		if coll.KeyedBy == nil {
			return false // a table: keyed by String
		}
		t = c.fieldType(coll.KeyedBy)
	}
}
