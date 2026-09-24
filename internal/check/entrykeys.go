package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// keyedEntryKey types `entry t.k`'s k as the key field and reports it written twice (TYPES.md §9.3).
func (c *checker) keyedEntryKey(env *env, d *syntax.EntryDecl, table *object, key *types.Field) {
	v := c.entryKeyValue(env, d.Key, key.Type)
	if v == nil {
		return
	}
	written := c.writtenKeys(table, key)
	if first, dup := written[v.CanonText()]; dup {
		c.report(env, diag.E3102.AtKey(env.span(d.Key), v, first))
		return
	}
	written[v.CanonText()] = env.span(d.Key)
}

// entryKeyValue is an entry key as a value of the key type, nil when it has none statically.
func (c *checker) entryKeyValue(env *env, k syntax.EntryKey, kt types.Type) value.Value {
	switch k := k.(type) {
	case *syntax.IntLit:
		if c.expr(env, k, kt).Kind() != types.Int || kt.Base().Kind() != types.Int || !k.Value.IsInt64() {
			return nil
		}
		return &value.Int{V: k.Value.Int64(), T: kt}
	case *syntax.Ident:
		return c.wordKey(env, k, kt)
	}
	return nil
}

// wordKey is a WORD entry key: a String key, an enum member (E3003 when it is none), the key
// of a ref's entry (checked by verify); anything else takes no word (E3002).
func (c *checker) wordKey(env *env, k *syntax.Ident, kt types.Type) value.Value {
	switch x := kt.Base().(type) {
	case *types.EnumType:
		if c.memberObject(x, k.Name) == nil {
			c.report(env, diag.E3003.At(env.span(k), kt, diag.KindMember, k.Name))
			return nil
		}
	case *types.RefType:
	default:
		if kt.Base().Kind() != types.String {
			c.report(env, diag.E3002.At(env.span(k), kt, types.StringType))
			return nil
		}
	}
	return &value.Str{V: k.Name, T: kt}
}

// checkListKeys compares the keys written in every keyed-list literal of p (TYPES.md §9.3).
func (c *checker) checkListKeys(p *pkgState) {
	for _, o := range p.all {
		if o.kind != ObjLet || o.decl.(*syntax.LetDecl).Value == nil {
			continue
		}
		if _, keyed, ok := collectionElem(c.letType(o)); ok && keyed != nil {
			c.writtenKeys(o, keyed)
		}
	}
}

// writtenKeys are the keys written in a keyed list's literal, by text, at their first place;
// a literal key written twice is E3102 at the second, and breaks the let.
func (c *checker) writtenKeys(table *object, key *types.Field) map[string]source.Span {
	if keys, done := c.listKeys[table]; done {
		return keys
	}
	keys := map[string]source.Span{}
	c.listKeys[table] = keys
	list, ok := table.decl.(*syntax.LetDecl).Value.(*syntax.ListLit)
	if !ok {
		return keys
	}
	env := c.declEnv(table)
	for _, el := range list.Elems {
		v, at := literalKey(el, key)
		if v == nil {
			continue
		}
		if first, dup := keys[v.CanonText()]; dup {
			c.report(env, diag.E3102.AtKey(env.span(at), v, first))
			continue
		}
		keys[v.CanonText()] = env.span(at)
	}
	return keys
}

// literalKey is a literal element's key field written as an integer, a string or a member or key name.
func literalKey(el syntax.Expr, key *types.Field) (value.Value, syntax.Expr) {
	lit, ok := el.(*syntax.BraceLit)
	if !ok {
		return nil, nil
	}
	for _, it := range lit.Items {
		if f, isField := it.(*syntax.FieldItem); isField && f.Name.Name == key.Name {
			return writtenKey(f.Value, key.Type), f.Value
		}
	}
	return nil, nil
}

// writtenKey is the value of a key written as a literal, or a bare member or key name; nil for
// anything computed.
func writtenKey(v syntax.Expr, kt types.Type) value.Value {
	switch v := v.(type) {
	case *syntax.IntLit:
		if v.Value.IsInt64() {
			return &value.Int{V: v.Value.Int64(), T: kt}
		}
	case syntax.StrLit:
		if interpolationFree(v) {
			return &value.Str{V: constText(v), T: kt}
		}
	case *syntax.IdentExpr:
		if k := kt.Base().Kind(); k == types.Enum || k == types.Ref {
			return &value.Str{V: v.Name, T: kt}
		}
	default:
	}
	return nil
}
