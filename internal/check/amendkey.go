package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// keyKept is E1913 at a literal key field, of the element whole or alone, naming another key than its keyed-list segment; on holds each segment's container; a computed key or a position waits for evaluation (EVALUATION.md §9.3).
func (c *checker) keyKept(env *env, a *syntax.Amendment, on []types.Type, path string) {
	i, stated := lineKey(a, on)
	if stated == nil {
		return
	}
	kt := keyedList(on[i]).KeyedBy.Type
	want, got := c.segmentKey(a.Path[i], kt), c.staticKey(stated, kt)
	if want == nil || got == nil || sameKey(want, got) || !c.fitsKey(stated, got, kt) {
		return
	}
	c.report(env, diag.E1913.At(env.span(stated), path, got, want))
}

// fitsKey reports a stated key v of key type kt statically: typed as kt or its leaf, no entry or member of another collection or enum, a static key of a ref's target; else its own finding comes (TYPES.md §6.2, §10.3).
func (c *checker) fitsKey(e syntax.Expr, v value.Value, kt types.Type) bool {
	e = inner(e)
	if t := c.info.Types[e]; t == nil || !c.assignable(t, kt) && !c.leafValue(t, kt) {
		return false
	}
	if o := c.keyObject(e); o != nil {
		return c.ownKeyObject(o, kt)
	}
	rt, isRef := kt.Base().(*types.RefType)
	if !isRef {
		return true
	}
	keys := c.staticKeys(rt.Target)
	return keys != nil && keys.byName[v.CanonText()] != nil
}

// keyObject is the declaration a key written as a name resolved to, nil for any other key.
func (c *checker) keyObject(e syntax.Expr) *object {
	var o Object
	switch x := e.(type) {
	case *syntax.IdentExpr:
		o = c.info.Uses[x]
	case *syntax.SelectorExpr:
		o = c.info.NameUses[x.Name]
	}
	obj, _ := o.(*object)
	return obj
}

// ownKeyObject reports an entry of the let a ref key type kt targets, a member of its enum, or
// any other name.
func (c *checker) ownKeyObject(o *object, kt types.Type) bool {
	if o.kind == ObjMember {
		return o.owner != nil && (c.assignable(o.owner, kt) || c.leafValue(o.owner, kt))
	}
	if o.kind != ObjEntry {
		return true
	}
	rt, isRef := kt.Base().(*types.RefType)
	return isRef && o.parent != nil && rt.Target.Kind == types.CollLet && len(rt.Target.FieldPath) == 0 &&
		rt.Target.Pkg == o.parent.pkg && rt.Target.Name == o.parent.name
}

// lineKey is the key field a line states for the element a keyed list's segment names, with
// that segment's index: its value's field when the segment is the last, the value itself when
// the last segment is the key field; nil when it states none.
func lineKey(a *syntax.Amendment, on []types.Type) (int, syntax.Expr) {
	last := len(a.Path) - 1
	if lt := keyedList(on[last]); lt != nil {
		_, stated := literalKey(inner(a.Value), lt.KeyedBy)
		return last, stated
	}
	if last == 0 || a.Path[last].Name == nil {
		return 0, nil
	}
	if lt := keyedList(on[last-1]); lt != nil && a.Path[last].Name.Name == lt.KeyedBy.Name {
		return last - 1, a.Value
	}
	return 0, nil
}

// keyedList is t's list type when it is a keyed list, else nil.
func keyedList(t types.Type) *types.ListType {
	if lt, ok := t.Base().(*types.ListType); ok && lt.KeyedBy != nil {
		return lt
	}
	return nil
}

// segmentKey is the key a segment names, as a value of the key type; nil for a position or a
// computed key.
func (c *checker) segmentKey(seg *syntax.AmendSegment, kt types.Type) value.Value {
	if seg.Name != nil {
		return &value.Str{V: seg.Name.Name, T: kt}
	}
	if seg.Key == nil {
		return nil
	}
	return c.staticKey(seg.Key, kt)
}

// staticKey is a key written as an integer or string literal, or a name the checker resolved to
// a key, an entry or a member; nil for anything else, a name in scope included (DECISIONS 494).
func (c *checker) staticKey(e syntax.Expr, kt types.Type) value.Value {
	e = inner(e)
	if c.lexError(e) {
		return nil
	}
	switch x := e.(type) {
	case *syntax.IntLit:
		if x.Value.IsInt64() {
			return &value.Int{V: x.Value.Int64(), T: kt}
		}
		return nil
	case syntax.StrLit:
		if interpolationFree(x) {
			return &value.Str{V: constText(x), T: kt}
		}
		return nil
	}
	if name, ok := c.keyName(e); ok {
		return &value.Str{V: name, T: kt}
	}
	return nil
}

// sameKey reports two static keys that are one key, or of two kinds, which a type finding has
// refused already.
func sameKey(a, b value.Value) bool {
	_, aInt := a.(*value.Int)
	_, bInt := b.(*value.Int)
	return aInt != bInt || a.CanonText() == b.CanonText()
}
