package check

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// checkLayers checks every layer file of p, active or not, a second of one name too (EVALUATION.md §9.1).
func (c *checker) checkLayers(p *pkgState) {
	first := map[string]source.Span{}
	for _, f := range p.files {
		if f.FileKind != syntax.FileLayer || f.Layer == nil {
			continue
		}
		o := c.newObject(ObjLayer, f.Layer.Name, p, f.Layer, f)
		c.info.Defs[f.Layer] = o
		if at, dup := first[f.Layer.Name]; dup {
			c.deliver(p, origin{file: f}, diag.E1906.At(f.Span(f.Layer), p.path, f.Layer.Name, at).Report)
		} else {
			first[f.Layer.Name] = f.Span(f.Layer)
		}
		seen := amendPaths{}
		for _, b := range f.Amends {
			c.amendBlock(c.fileEnv(p, f, o), b, seen)
		}
	}
}

// amendBlock is `amend v { path: value … }`: v a let of the layer's package (E1909 for another
// package's, E1902 for anything but a let, E2102 when unknown).
func (c *checker) amendBlock(env *env, b *syntax.AmendBlock, seen amendPaths) {
	name := b.Target.Name
	o := env.pkg.names[name]
	switch {
	case o == nil && c.declaredElsewhere(env.pkg, name):
		c.report(env, diag.E1909.At(env.span(b.Target), name))
		return
	case o == nil && c.universe[name] != nil:
		c.report(env, diag.E1902.At(env.span(b.Target), name))
		return
	case o == nil:
		c.unknownName(env, b.Target, name)
		return
	case o.kind != ObjLet:
		c.report(env, diag.E1902.At(env.span(b.Target), name))
		return
	}
	c.useLet(env, b.Target, o)
	for _, a := range b.Items {
		c.amendment(env, o, a, seen)
	}
}

// declaredElsewhere reports a name that another loaded package declares.
func (c *checker) declaredElsewhere(p *pkgState, name string) bool {
	for _, q := range c.sorted {
		if _, ok := q.names[name]; ok && q != p {
			return true
		}
	}
	return false
}

// amendment types one `path: value`: the path resolved against the let's static type (E1905),
// set once and not overlapping another in the layer (E1908), the value checked at the path.
func (c *checker) amendment(env *env, let *object, a *syntax.Amendment, seen amendPaths) {
	w := &amendWalk{let: let, decl: let, t: c.letType(let), plain: c.letType(let).Base().Kind() != types.Optional, bind: &bindCtx{field: c.letType(let)}}
	for i, seg := range a.Path {
		if i == 0 && seg.Name != nil {
			w.text = append(w.text, seg.Name.Name)
		} else {
			w.text = append(w.text, segmentText(env, seg))
		}
		container := unwrapOptional(w.t)
		if !c.amendSegment(env, w, seg) {
			c.expr(env, a.Value, types.ErrorType)
			return
		}
		w.canon = append(w.canon, c.canonSegment(env, container, seg))
	}
	c.overlap(env, a, let, amendPath{text: w.path(), canon: w.canon}, seen)
	ve := env.storing(w.decl, a.Value)
	ve.bind = w.bind
	c.expr(ve, a.Value, w.t)
}

// amendWalk is a path being resolved: the type reached, the path's text so far and its
// canonical segments, and the field names from the let while every segment is a record field
// (the collection a key belongs to).
type amendWalk struct {
	let    *object
	decl   *object // the let, then the last field on the path: the declaration typing what it reached (API.md F12)
	t      types.Type
	text   []string
	canon  []string
	fields []string
	plain  bool
	bind   *bindCtx // where evaluation reads a literal's type arguments at the path so far
	via    *object  // the field the last segment named, nil after any other segment
}

func (w *amendWalk) path() string { return strings.Join(w.text, "") }

// segmentText is a path segment as a path prints it: `.f` (the first bare), `[k]`, `[#n]`.
func segmentText(env *env, seg *syntax.AmendSegment) string {
	switch {
	case seg.Name != nil:
		return dot + seg.Name.Name
	case seg.Position != nil:
		return openBracket + hash + seg.Position.Value.String() + closeBracket
	}
	return openBracket + env.written(seg.Key) + closeBracket
}

// amendSegment steps w over one segment, through an optional but never a ref (EVALUATION.md §9.2).
func (c *checker) amendSegment(env *env, w *amendWalk, seg *syntax.AmendSegment) bool {
	t := unwrapOptional(w.t)
	var next types.Type
	ok := false
	switch {
	case t.Base().Kind() == types.Ref:
	case seg.Name != nil:
		next, ok = c.amendField(env, w, t, seg.Name)
		if !ok {
			return false
		}
		w.plain = w.plain && t.Base().Kind() == types.Record
		w.fields = append(w.fields, seg.Name.Name)
	case seg.Position != nil:
		c.info.Types[seg.Position] = types.IntType
		next, ok = c.atPosition(env, w, t, seg)
	default:
		next, ok = c.amendKey(env, w, t, seg.Key)
	}
	if !ok {
		c.report(env, diag.E1905.AtField(env.span(seg), w.path()))
		return false
	}
	if seg.Name == nil || next.Base().Kind() == types.Optional {
		w.plain = false
	}
	w.bind = w.bind.step(t, seg, next)
	w.t, w.via = next, c.fieldStepped(seg)
	return true
}

// amendField is `.f`: a record's field (a case's for a variant), or an entry of a table or a
// keyed list whose leaf key is a String or a table id (stringLeaf), named by its object when the
// let's keys are static; through an input it is E3313.
func (c *checker) amendField(env *env, w *amendWalk, t types.Type, n *syntax.Ident) (types.Type, bool) {
	sel, ft := c.selectOn(t, n.Name)
	if sel == nil || sel.Kind == SelBuiltinMember {
		if v, ok := t.Base().(*types.VariantType); ok {
			return c.caseField(env, w, v, n)
		}
		c.report(env, diag.E1905.AtField(env.span(n), w.path()))
		return nil, false
	}
	if sel.Kind == SelEntry {
		c.amendEntry(w, t, n)
	}
	if o, ok := sel.Obj.(*object); ok && o != nil {
		c.info.NameUses[n] = o
		if o.kind == ObjField && o.field.Input != nil {
			c.report(env, diag.E3313.At(env.span(n), o.name))
			return nil, false
		}
		if o.kind == ObjField {
			w.decl = o
			return o.field.Type, true
		}
	}
	return ft, true
}

// amendEntry records `.k` naming an entry: its object when the let's keys are static, else its collection (DECISIONS 470).
func (c *checker) amendEntry(w *amendWalk, t types.Type, n *syntax.Ident) {
	if len(w.fields) == 0 && w.let.keys != nil && w.let.keys.byName[n.Name] != nil {
		c.info.NameUses[n] = w.let.keys.byName[n.Name]
		return
	}
	if coll := c.amendColl(w, t); coll != nil {
		c.info.NameKeys[n] = coll
	}
}

// amendColl is the collection `.k` steps into: the let's at its field path, an optional let's own before any step, else the last field's (TYPES.md §9.1, DECISIONS 487).
func (c *checker) amendColl(w *amendWalk, t types.Type) *types.Collection {
	if w.plain || len(w.canon) == 0 {
		return c.fieldColl(w.let, w.fields, t)
	}
	if w.via == nil {
		return nil
	}
	if owner := requiredRecord(w.via.owner); owner != nil {
		return c.heldCollection(owner, w.via.field)
	}
	return nil
}

// fieldStepped is the record field a `.f` segment named, nil for any other segment.
func (c *checker) fieldStepped(seg *syntax.AmendSegment) *object {
	if seg.Name == nil {
		return nil
	}
	if o, ok := c.info.NameUses[seg.Name].(*object); ok && o.kind == ObjField {
		return o
	}
	return nil
}

// caseField is `.f` on a variant: the field of the case holding it, checked when applied.
func (c *checker) caseField(env *env, w *amendWalk, v *types.VariantType, n *syntax.Ident) (types.Type, bool) {
	for _, ct := range v.Cases {
		if f := fieldNamed(ct.Fields, n.Name); f != nil {
			c.info.NameUses[n] = c.fieldObjects[f]
			w.decl = c.fieldObjects[f]
			return f.Type, true
		}
	}
	c.report(env, diag.E1905.AtField(env.span(n), w.path()))
	return nil, false
}

// atPosition is `[#n]` on any list, keyed list, table, map or dependent map; W1912 off a plain list (EVALUATION.md §9.2).
func (c *checker) atPosition(env *env, w *amendWalk, t types.Type, seg *syntax.AmendSegment) (types.Type, bool) {
	next, ok := elemAtPosition(t)
	if ok && !plainList(t) {
		c.warn(env, diag.W1912.At(env.span(seg), w.path()))
	}
	return next, ok
}

// elemAtPosition is the element or entry type `[#n]` reaches.
func elemAtPosition(t types.Type) (types.Type, bool) {
	switch x := t.Base().(type) {
	case *types.ListType:
		return x.Elem, true
	case *types.TableType:
		return x.Elem, true
	case *types.MapType:
		return x.Value, true
	case *types.DepMapType:
		return x.Value, true
	}
	return nil, false
}

// amendKey is `[k]` on a table, keyed list, map, dependent map or list; a bare key name is a key (TYPES.md §9.3).
func (c *checker) amendKey(env *env, w *amendWalk, t types.Type, k syntax.Expr) (types.Type, bool) {
	switch x := t.Base().(type) {
	case *types.ListType:
		if x.KeyedBy == nil {
			c.expr(env, k, types.IntType)
			return x.Elem, true
		}
		c.entryKey(c.keyedBy(env, k, x.KeyedBy), w, k, x.KeyedBy.Type)
		return x.Elem, true
	case *types.TableType:
		c.entryKey(env, w, k, types.StringType)
		return x.Elem, true
	case *types.MapType:
		c.expr(env.storing(w.decl, k), k, x.Key)
		return x.Value, true
	case *types.DepMapType: // EVALUATION.md §9.2, DECISIONS 409
		c.expr(env, k, &types.RefType{Target: x.Coll})
		return x.Value, true
	}
	c.synth(env, k)
	return nil, false
}

// entryKey types an amend key of the collection w reached: a bare name no scope or key type has is its key (TYPES.md §4.1, DECISIONS 318, 470).
func (c *checker) entryKey(env *env, w *amendWalk, k syntax.Expr, key types.Type) {
	if c.memberFirst(env, k, key) || c.bareEntryKey(env, w, k, key) || c.scopedEntryKey(env, w, k, key) || c.leafEntryKey(env, k, key) {
		return
	}
	c.expr(env, k, key)
}

// bareEntryKey types a bare name no scope or key type has as a key of the collection w reached (TYPES.md §4.1, DECISIONS 318, 470).
func (c *checker) bareEntryKey(env *env, w *amendWalk, k syntax.Expr, key types.Type) bool {
	id, isName := k.(*syntax.IdentExpr)
	if !isName || c.lookup(env, id.Name) != nil {
		return false
	}
	if o, _ := c.inExpected(unwrapUnion(key), id.Name); o != nil {
		return false
	}
	coll := c.amendColl(w, unwrapOptional(w.t))
	if coll == nil || !c.mayKey(coll, id.Name) {
		return false
	}
	c.info.Keys[k] = coll
	c.info.Types[k] = key
	return true
}

// scopedEntryKey types an amend key written as a name in scope whose value fits neither the key nor its chain's leaf as an expression's key is: a ref into the collection w reached is a key; E3027 for any other value or a name that is not a value, E3403 for an optional of a key (TYPES.md §4.1, §6.5, STDLIB.md §5, DECISIONS 318, 490).
func (c *checker) scopedEntryKey(env *env, w *amendWalk, k syntax.Expr, key types.Type) bool {
	id, isName := k.(*syntax.IdentExpr)
	if !isName {
		return false
	}
	o := c.lookup(env, id.Name)
	if o == nil {
		return false
	}
	if c.notValueName(env, k, key) {
		return true
	}
	if found, silent := c.scopedType(o); silent || c.assignable(found, key) || c.leafValue(found, key) {
		return false
	}
	own := c.amendColl(w, unwrapOptional(w.t))
	if t := c.synth(env, k); t.Kind() != types.Error && !c.ownRef(t, own) {
		c.notKey(env, k, key, t, own)
	}
	return true
}

// leafEntryKey types an amend key of a list keyed by a ref against its chain's leaf, as one level types it: a value name not of the key's own type, a context-dependent operand, or a link of a chain whose type fits the leaf; false for any other key (TYPES.md §4.1, DECISIONS 456).
func (c *checker) leafEntryKey(env *env, k syntax.Expr, key types.Type) bool {
	if key.Base().Kind() != types.Ref {
		return false
	}
	switch x := inner(k).(type) {
	case *syntax.IdentExpr:
		if o := c.lookup(env, x.Name); o == nil || c.valueOf(o, key) {
			return false
		}
	case *syntax.SelectorExpr, *syntax.IndexExpr, *syntax.ForceExpr, *syntax.CallExpr: // typed without their expected type, a call's included
		t := c.expr(env, k, nil)
		c.accept(env, k, t, c.selectedKey(t, key))
		return true
	default:
		if literalOf(k) != nil || !c.contextDependent(env, k) {
			return false
		}
	}
	leaf := c.refLeaf(key)
	if leaf == nil {
		return false
	}
	c.expr(env, k, leaf)
	return true
}

// selectedKey is the type an amend key of type t, a link of a chain, is accepted against when the key is a ref: the key for a value of it or any ref (a ref into another collection is refused against the key, STDLIB.md §5), else its chain's leaf, as one level types it (DECISIONS 456).
func (c *checker) selectedKey(t, key types.Type) types.Type {
	if c.assignable(t, key) || optElem(t).Base().Kind() == types.Ref {
		return key
	}
	return c.keyedAs(key)
}
