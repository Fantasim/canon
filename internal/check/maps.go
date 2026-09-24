package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// mapLit is a map literal against {K: V} or a dependent map (TYPES.md §5.2).
func (c *checker) mapLit(env *env, e *syntax.BraceLit, key, value, t types.Type) types.Type {
	c.info.Literals[e] = LitMap
	for _, it := range e.Items {
		switch it := it.(type) {
		case *syntax.MapItem:
			c.expr(env, it.Key, key)
			c.expr(env, it.Value, value)
		case *syntax.FieldItem:
			c.identKey(env, it.Name, key)
			c.expr(env, it.Value, value)
		default:
			c.report(env, diag.E3320.At(env.span(it), itemKind(it), diag.KindMap))
			c.itemsAlone(env, []syntax.BraceItem{it})
		}
	}
	c.duplicateKeys(env, e.Items)
	return t
}

// identKey is `name:` in a map literal: contextual against the key type, then in scope; a
// String or integer key type is E3304 (write "name": or (name):).
func (c *checker) identKey(env *env, n *syntax.Ident, key types.Type) {
	switch key.Base().Kind() {
	case types.String, types.Int:
		c.report(env, diag.E3304.At(env.span(n), n.Name))
		return
	default:
	}
	if o, _ := c.inExpected(unwrapUnion(key), n.Name); o != nil {
		c.info.NameUses[n] = o
		c.deprecatedUse(env, n, o)
		return
	}
	if o := c.lookup(env, n.Name); o != nil && c.valueOf(o, key) {
		c.info.NameUses[n] = o
		c.dependsOn(env, o)
		return
	}
	if c.dynamicKeys(key) != nil || unwrap(key).Kind() == types.DepUnion {
		return
	}
	c.unknownName(env, n, n.Name)
}

// duplicateKeys is E3322 for a key written twice as a constant or an identifier (TYPES.md §5.2).
func (c *checker) duplicateKeys(env *env, items []syntax.BraceItem) {
	seen := map[string]bool{}
	for _, it := range items {
		text, at, ok := staticKey(it)
		if !ok {
			continue
		}
		if seen[text] {
			c.report(env, diag.E3322.At(env.span(at), rawValue(text)))
			continue
		}
		seen[text] = true
	}
}

// staticKey is the canonical text of a key written as an identifier or a constant literal.
func staticKey(it syntax.BraceItem) (string, syntax.Node, bool) {
	switch it := it.(type) {
	case *syntax.FieldItem:
		return it.Name.Name, it.Name, true
	case *syntax.MapItem:
		switch k := inner(it.Key).(type) {
		case *syntax.IntLit:
			return k.Value.String(), it.Key, true
		case *syntax.IdentExpr:
			return k.Name, it.Key, true
		case syntax.StrLit:
			if interpolationFree(k) {
				return types.QuoteString(constText(k)), it.Key, true
			}
		}
	}
	return "", nil, false
}

// mapComp is `{k: v for …}` (TYPES.md §5.2).
func (c *checker) mapComp(env *env, e *syntax.BraceLit, want types.Type) types.Type {
	c.info.Literals[e] = LitMapComp
	inner := c.clauses(env, e.Clauses)
	item, ok := e.Items[0].(*syntax.MapItem)
	if !ok {
		c.itemsAlone(inner, e.Items)
		return types.ErrorType
	}
	switch w := unwrap(want).(type) {
	case *types.MapType:
		c.expr(inner, item.Key, w.Key)
		c.expr(inner, item.Value, w.Value)
		return w
	case nil:
		k, v := c.synth(inner, item.Key), c.synth(inner, item.Value)
		if !mapKey(k) {
			c.report(env, diag.E3011.At(env.span(item.Key), k))
		}
		return &types.MapType{Key: k, Value: v}
	}
	c.synth(inner, item.Key)
	c.synth(inner, item.Value)
	c.report(env, diag.E3002.At(env.span(e), want, &types.MapType{Key: types.AnyType, Value: types.AnyType}))
	return types.ErrorType
}
