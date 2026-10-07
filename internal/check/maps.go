package check

import (
	"slices"

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
	c.wireKeyClash(env, e.Items, key)
	return t
}

// identKey is `name:` in a map literal: contextual, in scope, else symbolic (dynamic or dependent keys, TYPES.md §4.1); E3304 on String or Int keys.
func (c *checker) identKey(env *env, n *syntax.Ident, key types.Type) {
	switch key.Base().Kind() {
	case types.String, types.Int:
		c.report(env, diag.E3304.At(env.span(n), n.Name))
		return
	default:
	}
	if !mapKey(key) { // its map type was refused, E3011 (TYPES.md §1)
		return
	}
	if !c.predicateIt(env, n.Name) && c.keyStepOne(env, n, key) {
		return
	}
	o := c.lookup(env, n.Name)
	if o == nil {
		if !c.strayIt(env, n, n.Name) && c.dynamicKeys(key) == nil {
			c.unknownName(env, n, n.Name)
		}
		return
	}
	c.info.NameUses[n] = o
	c.dependsOn(env, o)
	if !c.valueOf(o, key) { // static keys too: E3027, not E2102 (TYPES.md §4.1, DECISIONS 334)
		c.keyInScope(env, n, o, key)
	}
}

// keyStepOne is step 1 for `name:`: a member or static key of the key type, or a dependent key (TYPES.md §4.1).
func (c *checker) keyStepOne(env *env, n *syntax.Ident, key types.Type) bool {
	if o, _ := c.inExpected(unwrapUnion(key), n.Name); o != nil {
		c.info.NameUses[n] = o
		c.deprecatedUse(env, n, o)
		return true
	}
	if fn := depFunc(unwrapUnion(key)); fn != nil {
		c.dependentKey(env, n, key, fn)
		return true
	}
	return false
}

// dependentKey is `name:` against a dependent key type: a name in scope, else symbolic (TYPES.md §11.4).
func (c *checker) dependentKey(env *env, n *syntax.Ident, key types.Type, fn *types.TypeFunc) {
	o := c.dependentName(env, n, n.Name, fn)
	if o == nil {
		o = c.scopedKey(env, n.Name, fn)
		c.keyInScope(env, n, o, key)
	}
	if o == nil {
		c.strayDependentIt(env, n, n.Name, fn)
		return
	}
	c.info.NameUses[n] = o
	c.dependsOn(env, o)
}

// keyInScope is E3027 (E3403 for an optional ref) for a map key `n:` naming something in scope (TYPES.md §4.1).
func (c *checker) keyInScope(env *env, n *syntax.Ident, o *object, key types.Type) {
	if o == nil {
		return
	}
	member := c.memberKey(key)
	if !isValue(o.kind) && member {
		c.report(env, diag.E3027.AtNotValueMember(env.span(n), o.name, notValueKind(o), key))
		return
	}
	if !isValue(o.kind) {
		c.notValueKey(env, n, o)
		return
	}
	found, silent := c.scopedType(o)
	switch {
	case silent:
	case c.optionalOf(o, key):
		c.report(env, diag.E3403.At(env.span(n), key, found))
	case member:
		c.report(env, diag.E3027.AtValueMember(env.span(n), n.Name, key, found))
	default:
		c.report(env, diag.E3027.AtValue(env.span(n), n.Name, key, found))
	}
}

// memberKey reports a key type with no string form, whose E3027 says to write a member (DECISIONS 334).
func (c *checker) memberKey(key types.Type) bool {
	return c.keyTarget(key) == nil && depFunc(unwrapUnion(key)) == nil
}

// scopedType is the type of the value o names; silent for a value already in error (TYPES.md §1).
func (c *checker) scopedType(o *object) (types.Type, bool) {
	t := o.typ
	switch o.kind {
	case ObjLet:
		t = c.letType(o)
	case ObjConst:
		t = c.constType(o)
	default:
	}
	if t == nil || t.Kind() == types.Error {
		return nil, true
	}
	return staticView(t), false
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

// unionKey is a static literal-union map key: wire text, text as written, a literal or not (WIRE.md §5.8).
type unionKey struct {
	text, canon string
	lit         bool
}

// wireKeyClash is E3317 at the second of a literal and another key with one wire text (WIRE.md §5.8).
func (c *checker) wireKeyClash(env *env, items []syntax.BraceItem, key types.Type) {
	u, ok := key.Base().(*types.LitUnionType)
	if !ok {
		return
	}
	var seen []unionKey
	for _, it := range items {
		k, at, known := c.unionKeyOf(it, u)
		if !known {
			continue
		}
		i := slices.IndexFunc(seen, func(s unionKey) bool { return s.text == k.text && s.lit != k.lit })
		if i >= 0 {
			c.report(env, diag.E3317.At(env.span(at), rawValue(seen[i].canon), rawValue(k.canon), k.text))
			continue
		}
		seen = append(seen, k)
	}
}

// unionKeyOf is the unionKey of a string literal key, or of a name the union's other type gives (TYPES.md §13.2).
func (c *checker) unionKeyOf(it syntax.BraceItem, u *types.LitUnionType) (unionKey, syntax.Node, bool) {
	canon, at, ok := staticKey(it)
	if !ok {
		return unionKey{}, nil, false
	}
	switch k := it.(type) {
	case *syntax.FieldItem:
		text, known := c.nameKeyText(k.Name, u.Of)
		return unionKey{text: text, canon: canon}, at, known
	case *syntax.MapItem:
		if s, isStr := inner(k.Key).(syntax.StrLit); isStr {
			text := constText(s)
			return unionKey{text: text, canon: canon, lit: slices.Contains(u.Literals, text)}, at, true
		}
	}
	return unionKey{}, nil, false
}

// nameKeyText is the wire key of a name used as a key of type of: an enum member's wire value, or
// a ref's entry key (its member's wire value for a keyed list keyed by an enum).
func (c *checker) nameKeyText(n *syntax.Ident, of types.Type) (string, bool) {
	switch a := of.Base().(type) {
	case *types.EnumType:
		if o := c.memberObject(a, n.Name); o == nil || c.info.NameUses[n] != Object(o) {
			return "", false
		}
		return memberWire(a, n.Name)
	case *types.RefType:
		coll := c.coll(a)
		if c.noTarget(coll) || !c.namesKey(n, coll) { // a ref to nothing is the error type (TYPES.md §1)
			return "", false
		}
		if coll.KeyedBy != nil {
			if e, isEnum := coll.KeyedBy.Type.Base().(*types.EnumType); isEnum {
				return memberWire(e, n.Name)
			}
		}
		return n.Name, true
	}
	return "", false
}

// namesKey reports a name that is a key of coll, declared or symbolic (TYPES.md §4.1).
func (c *checker) namesKey(n *syntax.Ident, coll *types.Collection) bool {
	use := c.info.NameUses[n]
	if use == nil {
		return true
	}
	keys := c.staticKeys(coll)
	return keys != nil && use == Object(keys.byName[n.Name])
}

// memberWire is a member's wire value; none with @json(codes), E3002 in a literal union (TYPES.md §13.2).
func memberWire(e *types.EnumType, name string) (string, bool) {
	i := slices.IndexFunc(e.Members, func(m *types.Member) bool { return m.Name == name })
	if i < 0 || e.WireCodes {
		return "", false
	}
	return e.Members[i].Wire, true
}
