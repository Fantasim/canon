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
	c.info.NameUses[b.Target] = o
	c.dependsOn(env, o)
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
	w := &amendWalk{let: let, t: c.letType(let), plain: true}
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
	c.expr(env, a.Value, w.t)
}

// amendWalk is a path being resolved: the type reached, the path's text so far and its
// canonical segments, and the field names from the let while every segment is a record field
// (the collection a key belongs to).
type amendWalk struct {
	let    *object
	t      types.Type
	text   []string
	canon  []string
	fields []string
	plain  bool
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
		next, ok = atPosition(t)
	default:
		next, ok = c.amendKey(env, w, t, seg.Key)
	}
	if !ok {
		c.report(env, diag.E1905.AtField(env.span(seg), w.path()))
		return false
	}
	if seg.Name == nil {
		w.plain = false
	}
	w.t = next
	return true
}

// amendField is `.f`: a record's field (a case's for a variant), or an entry of a table or a
// String-keyed list, named by its object when the let's keys are static; through an input it
// is E3313.
func (c *checker) amendField(env *env, w *amendWalk, t types.Type, n *syntax.Ident) (types.Type, bool) {
	sel, ft := c.selectOn(t, n.Name)
	if sel == nil || sel.Kind == SelBuiltinMember {
		if v, ok := t.Base().(*types.VariantType); ok {
			return c.caseField(env, v, n, w.path())
		}
		c.report(env, diag.E1905.AtField(env.span(n), w.path()))
		return nil, false
	}
	if sel.Kind == SelEntry && len(w.fields) == 0 && w.let.keys != nil && w.let.keys.byName[n.Name] != nil {
		c.info.NameUses[n] = w.let.keys.byName[n.Name]
	}
	if o, ok := sel.Obj.(*object); ok && o != nil {
		c.info.NameUses[n] = o
		if o.kind == ObjField && o.field.Input != nil {
			c.report(env, diag.E3313.At(env.span(n), o.name))
			return nil, false
		}
		if o.kind == ObjField {
			return o.field.Type, true
		}
	}
	return ft, true
}

// caseField is `.f` on a variant: the field of the case holding it, checked when applied.
func (c *checker) caseField(env *env, v *types.VariantType, n *syntax.Ident, path string) (types.Type, bool) {
	for _, ct := range v.Cases {
		if f := fieldNamed(ct.Fields, n.Name); f != nil {
			c.info.NameUses[n] = c.fieldObjects[f]
			return f.Type, true
		}
	}
	c.report(env, diag.E1905.AtField(env.span(n), path))
	return nil, false
}

// atPosition is `[#n]`: any list, keyed list, table or map, by position.
func atPosition(t types.Type) (types.Type, bool) {
	switch x := t.Base().(type) {
	case *types.ListType:
		return x.Elem, true
	case *types.TableType:
		return x.Elem, true
	case *types.MapType:
		return x.Value, true
	}
	return nil, false
}

// amendKey is `[k]` on a table, keyed list, map or list; a bare key name is a key (TYPES.md §9.3).
func (c *checker) amendKey(env *env, w *amendWalk, t types.Type, k syntax.Expr) (types.Type, bool) {
	switch x := t.Base().(type) {
	case *types.ListType:
		if x.KeyedBy == nil {
			c.expr(env, k, types.IntType)
			return x.Elem, true
		}
		c.entryKey(env, w, k, x.KeyedBy.Type)
		return x.Elem, true
	case *types.TableType:
		c.entryKey(env, w, k, types.StringType)
		return x.Elem, true
	case *types.MapType:
		c.expr(env, k, x.Key)
		return x.Value, true
	}
	c.synth(env, k)
	return nil, false
}

// entryKey types an amend key of the collection w reached: a bare name no scope or key type
// has is its key when the path so far is the let's record fields.
func (c *checker) entryKey(env *env, w *amendWalk, k syntax.Expr, key types.Type) {
	id, isName := k.(*syntax.IdentExpr)
	if isName && w.plain && c.lookup(env, id.Name) == nil {
		if o, _ := c.inExpected(unwrapUnion(key), id.Name); o == nil {
			c.info.Keys[k] = c.fieldColl(w.let, w.fields, w.t)
			c.info.Types[k] = key
			return
		}
	}
	c.expr(env, k, key)
}
