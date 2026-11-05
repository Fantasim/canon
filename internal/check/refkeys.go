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
	_, ok := c.leafKey(t)
	return !ok
}

// leafKey is t's key at the leaf of its chain of ref keys: String for a table, Error for a target in error, false on a loop (WIRE.md §5.9).
func (c *checker) leafKey(t types.Type) (types.Type, bool) {
	ch := c.keyChain(t)
	return ch.Leaf, ch.End != types.ChainLoop
}

// leafField is the key field at the leaf of the chain of ref keys t starts, the first key down it that is not a ref; nil when t is no ref, or for a table, a target in error or a loop (DECISIONS 456).
func (c *checker) leafField(t types.Type) *types.Field {
	ch := c.keyChain(t)
	if ch.End != types.ChainLeaf {
		return nil
	}
	return ch.Field
}

// keyChain walks the chain of ref keys t starts as the checker resolves it (types.KeyChain, WIRE.md §5.9).
func (c *checker) keyChain(t types.Type) types.Chain {
	return types.KeyChain(t, chainLinks{c: c})
}

// chainLinks reads a chain's links as they resolve: a ref resolves when met, and one whose target resolved to nothing, or not yet, ends the chain as a target in error (TYPES.md §1, §10.2).
type chainLinks struct {
	c *checker
}

// Target is r's target once resolved, nil for a broken ref (brokenRef).
func (l chainLinks) Target(r *types.RefType) *types.Collection {
	if l.c.brokenRef(r) {
		return nil
	}
	return l.c.coll(r)
}

// KeyType is f's type once resolved (fieldType).
func (l chainLinks) KeyType(f *types.Field) types.Type {
	return l.c.fieldType(f)
}

// refLeaf is the type at the leaf of the chain of ref keys a ref key type starts, nil for a key that is no ref, a loop or a target in error (DECISIONS 456).
func (c *checker) refLeaf(key types.Type) types.Type {
	if key.Base().Kind() != types.Ref {
		return nil
	}
	leaf, ok := c.leafKey(key)
	if !ok || leaf.Kind() == types.Error {
		return nil
	}
	return leaf
}

// leafValue reports a value of type t that keys a list keyed by key, a ref, by its chain's leaf: as `ens[Q.a]` keys a list keyed by Q, `rs[Q.a]` keys one keyed by `ref ens` (TYPES.md §4.1, STDLIB.md §5, DECISIONS 456).
func (c *checker) leafValue(t, key types.Type) bool {
	leaf := c.refLeaf(key)
	return leaf != nil && c.assignable(t, leaf)
}

// keyedAs is the type a key of a list keyed by key is written as: its chain's leaf for a ref (refLeaf), else key itself (TYPES.md §4.1, DECISIONS 456).
func (c *checker) keyedAs(key types.Type) types.Type {
	if leaf := c.refLeaf(key); leaf != nil {
		return leaf
	}
	return key
}

// enumKey is the enum a key is, read at its chain's leaf, nil when it is no enum: its keys are member names, with no string form (TYPES.md §4.1, DECISIONS 334).
func (c *checker) enumKey(key types.Type) *types.EnumType {
	e, _ := unwrapUnion(c.keyedAs(key)).(*types.EnumType)
	return e
}

// keyedByInt reports a keyed list whose leaf key (leafKey) is an integer type: a ref into it is an integer literal (TYPES.md §4.1).
func (c *checker) keyedByInt(coll *types.Collection) bool {
	if coll.KeyedBy == nil {
		return false
	}
	leaf, ok := c.leafKey(c.fieldType(coll.KeyedBy))
	return ok && leaf.Base().Kind() == types.Int
}
