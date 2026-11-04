package check

import (
	"slices"

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
	_, ok := c.leafKey(t)
	return !ok
}

// leafKey is t's key at the leaf of its chain of ref keys: String for a table, Error for a target in error, false on a loop (WIRE.md §5.9).
func (c *checker) leafKey(t types.Type) (types.Type, bool) {
	_, leaf, end := c.keyChain(t)
	return leaf, end != endLoop
}

// leafField is the key field at the leaf of the chain of ref keys t starts, the first key down it that is not a ref; nil when t is no ref, or for a table, a target in error or a loop (DECISIONS 456).
func (c *checker) leafField(t types.Type) *types.Field {
	f, _, end := c.keyChain(t)
	if end != endLeaf {
		return nil
	}
	return f
}

// chainEnd is how a walk down a chain of ref keys ends.
type chainEnd uint8

// keyChain walks the chain of ref keys t starts: the last key field met (nil when t is no ref), the leaf's type (String for a table, Error for a target in error, nil on a loop) and how the walk ended (WIRE.md §5.9).
func (c *checker) keyChain(t types.Type) (*types.Field, types.Type, chainEnd) {
	var f *types.Field
	var seen []*types.Collection
	for {
		r, ok := t.Base().(*types.RefType)
		if !ok {
			return f, t, endLeaf
		}
		if c.brokenRef(r) {
			return f, types.ErrorType, endBroken
		}
		coll := c.coll(r)
		if slices.Contains(seen, coll) {
			return f, nil, endLoop
		}
		seen = append(seen, coll)
		if coll.KeyedBy == nil {
			return f, types.StringType, endTable
		}
		f = coll.KeyedBy
		t = c.fieldType(f)
	}
}

// keyedByInt reports a keyed list whose leaf key (leafKey) is an integer type: a ref into it is an integer literal (TYPES.md §4.1).
func (c *checker) keyedByInt(coll *types.Collection) bool {
	if coll.KeyedBy == nil {
		return false
	}
	leaf, ok := c.leafKey(c.fieldType(coll.KeyedBy))
	return ok && leaf.Base().Kind() == types.Int
}
