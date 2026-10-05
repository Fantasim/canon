package check

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// index is `x[i]` (TYPES.md §5.1, §9).
func (c *checker) index(env *env, x *syntax.IndexExpr) (types.Type, bool) {
	t, opt := c.recv(env, x.X)
	if t.Kind() == types.Error {
		c.indexAlone(env, x.Index)
		return t, opt
	}
	t, ok := c.optionalRecv(env, x.X, t, false, "")
	if !ok {
		c.indexAlone(env, x.Index)
		return types.ErrorType, opt
	}
	if r, isRef := t.Base().(*types.RefType); isRef {
		t = c.coll(r).Elem
	}
	if c.notDependent(env, x, t, indexText) {
		c.indexAlone(env, x.Index)
		return types.ErrorType, opt
	}
	if rng, isRange := x.Index.(*syntax.RangeExpr); isRange {
		return c.slice(env, x, t, rng), opt
	}
	return c.lookupIndex(env, x, t), opt
}

// indexAlone types an index whose receiver failed.
func (c *checker) indexAlone(env *env, i syntax.Expr) {
	if r, ok := i.(*syntax.RangeExpr); ok {
		c.sliceBounds(env, r)
		return
	}
	c.synth(env, i)
}

// slice is `xs[a..b]` and its forms on a list, keyed list, table or string (STDLIB.md §4.1, §7).
func (c *checker) slice(env *env, x *syntax.IndexExpr, t types.Type, r *syntax.RangeExpr) types.Type {
	c.sliceBounds(env, r)
	switch b := t.Base().(type) {
	case *types.ListType:
		return &types.ListType{Elem: b.Elem}
	case *types.TableType:
		return &types.ListType{Elem: b.Elem}
	}
	if t.Base().Kind() == types.String {
		return types.StringType
	}
	c.report(env, diag.E3007.AtBinary(env.span(x), indexText, t, types.RangeType))
	return types.ErrorType
}

// sliceBounds types the bounds of a slice index as Int.
func (c *checker) sliceBounds(env *env, r *syntax.RangeExpr) {
	c.info.Types[r] = types.RangeType
	for _, b := range []syntax.Expr{r.Lo, r.Hi} {
		if b != nil {
			c.expr(env, b, types.IntType)
		}
	}
}

// lookupIndex is a single index: an Int position of a plain list, a key of a keyed list or a
// table (a ref of it too), a key of a map; a Range value slices.
func (c *checker) lookupIndex(env *env, x *syntax.IndexExpr, t types.Type) types.Type {
	switch b := t.Base().(type) {
	case *types.ListType:
		if b.KeyedBy == nil {
			return c.positional(env, x, b)
		}
		c.keyValue(env, x.Index, x.X, b.KeyedBy.Type)
		return b.Elem
	case *types.TableType:
		c.keyValue(env, x.Index, x.X, types.StringType)
		return b.Elem
	case *types.MapType:
		c.expr(env, x.Index, b.Key)
		return staticView(b.Value)
	case *types.DepMapType:
		c.expr(env, x.Index, &types.RefType{Target: b.Coll})
		return staticView(b.Value)
	}
	it := c.synth(env, x.Index)
	c.report(env, diag.E3007.AtBinary(env.span(x), indexText, t, it))
	return types.ErrorType
}

// positional is `xs[i]` on a plain list: i an Int, or a Range value for a slice.
func (c *checker) positional(env *env, x *syntax.IndexExpr, l *types.ListType) types.Type {
	it := c.synth(env, x.Index)
	switch it.Base().Kind() {
	case types.Int, types.Error:
		return l.Elem
	case types.Range:
		return &types.ListType{Elem: l.Elem}
	default:
	}
	c.report(env, diag.E3002.At(env.span(x.Index), types.IntType, it))
	return types.ErrorType
}

// keyValue types a key (`xs[k]`, get, find, hasKey): KT, or a ref into recv's own collection (STDLIB.md §5).
func (c *checker) keyValue(env *env, k, recv syntax.Expr, key types.Type) {
	if c.bareKey(env, k, recv, key) || c.notValueName(env, k) {
		return
	}
	t := c.synth(env, k)
	if t.Kind() == types.Error || c.assignable(t, key) || c.refOf(t, recv) {
		return
	}
	if id, isName := k.(*syntax.IdentExpr); isName && t.Base().Kind() != types.Ref { // TYPES.md §4.1; a ref is STDLIB.md §5's E3002
		c.report(env, diag.E3027.AtValue(env.span(k), id.Name, key, t))
		return
	}
	c.report(env, diag.E3002.At(env.span(k), key, t))
}

// notValueName is E3027 `notValue` for a key written as a name in scope that is not a value (TYPES.md §4.1).
func (c *checker) notValueName(env *env, k syntax.Expr) bool {
	id, isName := k.(*syntax.IdentExpr)
	if !isName {
		return false
	}
	o := c.lookup(env, id.Name)
	if o == nil || isValue(o.kind) {
		return false
	}
	c.info.Uses[id] = o
	c.dependsOn(env, o)
	c.notValueKey(env, id, o)
	return true
}

// refOf reports a ref into the collection recv holds: a let's or a let path's, or a record field's.
func (c *checker) refOf(t types.Type, recv syntax.Expr) bool {
	r, isRef := t.Base().(*types.RefType)
	if !isRef || recv == nil {
		return false
	}
	coll := c.keyedReceiver(recv)
	if id, isName := recv.(*syntax.IdentExpr); coll == nil && isName {
		coll = c.fieldReadColl(id)
	}
	return coll != nil && c.coll(r) == coll
}

// fieldReadColl is the collection a field named in its record body holds (TYPES.md §10.2 level 1).
func (c *checker) fieldReadColl(id *syntax.IdentExpr) *types.Collection {
	o, ok := c.info.Uses[id].(*object)
	if !ok || o.kind != ObjField {
		return nil
	}
	if owner, isRecord := o.owner.(*types.RecordType); isRecord {
		return c.fieldCollection(owner, o.field)
	}
	return nil
}

// bareKey types an unscoped name in a key position: a member, a key of recv's collection, else a value (TYPES.md §5.1).
func (c *checker) bareKey(env *env, x, recv syntax.Expr, key types.Type) bool {
	id, ok := x.(*syntax.IdentExpr)
	if !ok || c.lookup(env, id.Name) != nil {
		return false
	}
	if o, _ := c.inExpected(unwrapUnion(key), id.Name); o == nil {
		if coll := c.receiverColl(recv); coll != nil {
			c.info.Keys[x] = coll
			c.info.Types[x] = key
			return true
		}
	}
	c.expr(env, x, key)
	return true
}

// receiverColl is the collection an expression names: a let, or a path through record fields of one (TYPES.md §10.2).
func (c *checker) receiverColl(x syntax.Expr) *types.Collection {
	root, segs := pathParts(x)
	if root == nil {
		return nil
	}
	o, ok := c.info.Uses[root].(*object)
	if !ok || o.kind != ObjLet {
		return nil
	}
	var names []string
	t := c.letType(o)
	for _, s := range segs {
		sel := c.info.Selections[s]
		if sel == nil || sel.Kind != SelField {
			return nil
		}
		names = append(names, s.Name.Name)
		t = c.info.Types[s]
	}
	return c.fieldColl(o, names, t)
}

// fieldColl is the collection of type t at a let's record-field path, nil when t is none.
func (c *checker) fieldColl(o *object, names []string, t types.Type) *types.Collection {
	elem, keyed, isColl := collectionElem(t)
	if !isColl {
		return nil
	}
	kind := types.CollLet
	if isDefines(o) {
		kind = types.CollDefines
	}
	k := collKey{kind: kind, pkg: o.pkg, name: o.name, path: strings.Join(names, dot)}
	return c.intern(k, &types.Collection{Kind: kind, Pkg: o.pkg, Name: o.name, FieldPath: names, Elem: elem, KeyedBy: keyed, Local: o.local})
}
