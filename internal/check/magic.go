package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// position is where values of a type occur: as a table's entries, as the values of maps (their
// key types, each once), as a list's elements.
type position struct {
	table bool
	keys  []types.Type
	list  bool
}

// magicNames are a view's magic names in pos, typed (VIEWMODEL.md §3.4, log-2026-09-28 item 7).
func (c *checker) magicNames(p *pkgState, f *syntax.File, d *syntax.ViewDecl, target types.Type) map[string]*object {
	pos, lost := c.positionOf(target), c.lostFor(target)
	m := map[string]*object{}
	// `id` is String, `key` its maps' key type when they agree, `index` Int; a position lost to
	// an error is the error type, so nothing follows the error.
	add := func(name string, t types.Type) { m[name] = c.magicName(p, f, d, name, t) }
	switch {
	case pos.table:
		add(idMember, types.StringType)
	case lost.table:
		add(idMember, types.ErrorType)
	}
	switch {
	case len(pos.keys) == 1:
		add(keyName, pos.keys[0])
	case len(pos.keys) == 0 && len(lost.keys) > 0:
		add(keyName, types.ErrorType)
	}
	switch {
	case pos.list:
		add(indexMember, types.IntType)
	case lost.list:
		add(indexMember, types.ErrorType)
	}
	return m
}

// magicName is the object of a magic name of the node decl's texts: a local of type t.
func (c *checker) magicName(p *pkgState, f *syntax.File, decl syntax.Node, name string, t types.Type) *object {
	o := c.newObject(ObjLocal, name, p, decl, f)
	o.typ = t
	return o
}

// positionOf is where values of a view's target occur: a case where its variant does.
func (c *checker) positionOf(target types.Type) position {
	if c.positions == nil {
		c.collectPositions()
	}
	pos := c.positionIn(target)
	if ct, ok := target.(*types.CaseType); ok {
		v := c.positionIn(ct.Variant)
		pos.table, pos.list = pos.table || v.table, pos.list || v.list
		for _, k := range v.keys {
			pos.keys = addKeyType(pos.keys, k)
		}
	}
	return pos
}

// positionIn is the position recorded for t; a record is a table's entry where tableOf says so.
func (c *checker) positionIn(t types.Type) position {
	var pos position
	if found := c.positions[t]; found != nil {
		pos = *found
		pos.keys = slices.Clone(found.keys)
	}
	if r, ok := t.(*types.RecordType); ok {
		pos.table = pos.table || c.tableOf[r]
	}
	return pos
}

// collectPositions walks every written type and every top-level let's type, packages in path
// order, so the first key type met is the same on every run.
func (c *checker) collectPositions() {
	c.positions = map[types.Type]*position{}
	for _, p := range c.sorted {
		for _, f := range p.files {
			syntax.Inspect(f, func(n syntax.Node) bool {
				if t, ok := n.(syntax.Type); ok {
					c.lostPosition(p, t)
					c.walkPositions(c.info.TypeExprs[t])
				}
				return true
			})
		}
		for _, o := range p.all {
			if o.kind == ObjLet {
				c.walkPositions(c.letType(o))
			}
		}
	}
}

// lostPosition records a table, list or map of p whose element or value is in error: a view's
// target may have stood there.
func (c *checker) lostPosition(p *pkgState, n syntax.Type) {
	if !lostElement(c.info.TypeExprs[n]) {
		return
	}
	lost := c.lost[p]
	if lost == nil {
		lost = &position{}
		c.lost[p] = lost
	}
	switch n.(type) {
	case *syntax.TableType:
		lost.table = true
	case *syntax.ListType, *syntax.KeyedType:
		lost.list = true
	case *syntax.MapType, *syntax.DepMapType:
		lost.keys = []types.Type{types.ErrorType}
	}
}

// lostFor merges the positions lost where the target can sit: its package and the packages
// importing it (log-2026-09-28, U5 round 2).
func (c *checker) lostFor(target types.Type) position {
	var out position
	p := c.pkgs[targetPkg(target)]
	if p == nil {
		return out
	}
	for _, q := range c.sorted {
		if l := c.lost[q]; l != nil && (q == p || slices.Contains(q.imports, p)) {
			out.table, out.list = out.table || l.table, out.list || l.list
			out.keys = append(out.keys, l.keys...)
		}
	}
	return out
}

// targetPkg is the package declaring a view's target type.
func targetPkg(t types.Type) string {
	switch x := t.(type) {
	case *types.RecordType:
		return x.Pkg
	case *types.VariantType:
		return x.Pkg
	case *types.CaseType:
		return x.Variant.Pkg
	case *types.EnumType:
		return x.Pkg
	}
	return ""
}

// lostElement reports a collection type in error, or whose element or value is.
func lostElement(t types.Type) bool {
	if t == nil || t.Kind() == types.Error {
		return true
	}
	switch x := t.Base().(type) {
	case *types.TableType:
		return x.Elem.Kind() == types.Error
	case *types.ListType:
		return x.Elem.Kind() == types.Error
	case *types.MapType:
		return x.Value.Kind() == types.Error
	case *types.DepMapType:
		return x.Value.Kind() == types.Error
	}
	return false
}

// walkPositions records the positions t's collections give their elements and values.
func (c *checker) walkPositions(t types.Type) {
	if t == nil {
		return
	}
	switch x := t.Base().(type) {
	case *types.OptionalType:
		c.walkPositions(x.Elem)
	case *types.ListType:
		c.positionAt(x.Elem).list = true
		c.walkPositions(x.Elem)
	case *types.TableType:
		c.positionAt(x.Elem).table = true
		c.walkPositions(x.Elem)
	case *types.MapType:
		pos := c.positionAt(x.Value)
		pos.keys = addKeyType(pos.keys, x.Key)
		c.walkPositions(x.Value)
	case *types.DepMapType:
		pos := c.positionAt(x.Value)
		pos.keys = addKeyType(pos.keys, &types.RefType{Target: x.Coll})
		c.walkPositions(x.Value)
	default:
	}
}

// positionAt is the position of the values of type t (an optional stands for its element, an
// applied record for its record), made on first use.
func (c *checker) positionAt(t types.Type) *position {
	b := t.Base()
	if o, ok := b.(*types.OptionalType); ok {
		b = o.Elem.Base()
	}
	if a, ok := b.(*types.AppliedRecord); ok {
		b = a.Rec
	}
	pos := c.positions[b]
	if pos == nil {
		pos = &position{}
		c.positions[b] = pos
	}
	return pos
}

// addKeyType adds k to keys unless an identical type is there (TYPES.md §6.1).
func addKeyType(keys []types.Type, k types.Type) []types.Type {
	if slices.ContainsFunc(keys, func(x types.Type) bool { return types.Identical(x, k) }) {
		return keys
	}
	return append(keys, k)
}
