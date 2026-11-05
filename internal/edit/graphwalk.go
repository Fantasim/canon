package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// typedChild is a child as Resolve reads it: its canonical segment, value and static type, its
// node kind when it is an entry or element, keyless for an element whose key is not known (P8).
type typedChild struct {
	seg     Seg
	v       value.Value
	typ     types.Type
	kind    NodeKind
	keyed   bool
	keyless bool
}

// typedChildren passes each child of v, read in its static container type ct as walk.next
// reads it, to each.
func typedChildren(v value.Value, ct types.Type, each func(typedChild)) {
	switch x := v.(type) {
	case *value.Record:
		for i, f := range fieldsOf(x.T) {
			if i < len(x.Fields) && x.Fields[i] != nil {
				each(typedChild{seg: Seg{Kind: SegField, Name: f.Name}, v: x.Fields[i], typ: f.Type})
			}
		}
	case *value.List:
		listChildren(x, ct, each)
	case *value.Table:
		_, isTable := baseOf(ct).(*types.TableType)
		for _, e := range x.Entries {
			each(typedChild{seg: entrySeg(e.Ident.Key), v: e, typ: tableElem(ct, x), kind: NodeEntry, keyed: isTable})
		}
	case *value.Map:
		kt, vt, _ := mapTypes(ct)
		for i := 0; vt != nil && i < len(x.Keys); i++ {
			each(typedChild{seg: keySeg(x.Keys[i], kt), v: x.Vals[i], typ: vt})
		}
	}
}

// listChildren are a list's elements: by index, or by key, once each, in a keyed list (P1, P8).
func listChildren(l *value.List, ct types.Type, each func(typedChild)) {
	lt, ok := baseOf(ct).(*types.ListType)
	if !ok {
		return
	}
	seen := map[Seg]bool{}
	for i, e := range l.Elems {
		c := typedChild{seg: intSeg(int64(i)), v: e, typ: lt.Elem}
		if lt.KeyedBy != nil {
			c = keyedChild(c, lt.KeyedBy)
		}
		switch {
		case !c.keyed || c.keyless:
		case seen[c.seg]:
			continue // a duplicate key: Resolve reads the first
		default:
			seen[c.seg] = true
		}
		each(c)
	}
}

// keyedChild is c, an element of a list keyed by key, named by its key when known (P1, P8).
func keyedChild(c typedChild, key *types.Field) typedChild {
	c.kind, c.keyed = NodeElement, true
	if k := keyField(c.v, key); k != nil {
		c.seg = keySeg(k, key.Type)
	} else {
		c.keyless = true
	}
	return c
}

// nodeWalk is one let's walk in Resolve's types: the segments, values, trie positions (noPlace:
// not in the trie yet) and nearest candidate at or above each depth of its path (noPlace: none),
// and whether the let holds another let's value (readsLet).
type nodeWalk struct {
	ns      *nodeSet
	r       rootRef
	segs    []Seg
	vals    []value.Value
	pos     []int
	near    []int
	aliased bool
}

// below adds the entries and elements under v, of static type typ, as candidates.
func (nw *nodeWalk) below(v value.Value, typ types.Type) {
	if typ == nil {
		return
	}
	ct, err := (&walk{cur: v, typ: typ}).container()
	if err != nil {
		return
	}
	typedChildren(v, ct, func(c typedChild) {
		if c.keyless {
			nw.ns.fail(nw.r, nil)
			return
		}
		nw.segs, nw.vals, nw.pos = append(nw.segs, c.seg), append(nw.vals, c.v), append(nw.pos, noPlace)
		nw.near = append(nw.near, nw.near[len(nw.near)-1])
		nw.child(c, ct)
		nw.below(c.v, c.typ)
		n := len(nw.segs) - 1
		nw.segs, nw.vals, nw.pos, nw.near = nw.segs[:n], nw.vals[:n+1], nw.pos[:n+1], nw.near[:n+1]
	})
}

// child adds c, at the walk's depth and read in ct, as a candidate when it is an entry or
// element met where its identity can be at home (API.md R11, TYP-02).
func (nw *nodeWalk) child(c typedChild, ct types.Type) {
	rec, ok := c.v.(*value.Record)
	if !ok || rec.Ident == nil || !c.keyed {
		return
	}
	if under, home := nw.home(rec.Ident); home {
		nw.near[len(nw.near)-1] = nw.candidate(rec, c, ct, under)
	}
}

// home reports whether an entry at the walk's depth may be at home there, and the candidate it
// lies under: a let's or a load.defines table's under its let's root only; a field's under its
// owner's instance, itself a candidate when it has an identity; any other one anywhere (API.md R11).
func (nw *nodeWalk) home(id *value.Identity) (under int, ok bool) {
	d := len(nw.segs)
	under = nw.near[d-1]
	switch c := id.Coll; {
	case c == nil:
		return under, true
	case c.Kind == types.CollDefines || c.Kind == types.CollLet && c.Name != "":
		return under, d == 1 && nw.owns(c)
	case c.Kind != types.CollField || id.Owner == nil:
		return under, true
	case d < ownerUp:
		return under, false
	}
	owner, ok := nw.vals[d-ownerUp].(*value.Record)
	if !ok || !nw.ownedBy(owner, id.Owner, d-ownerUp) {
		return under, false
	}
	return under, owner.Ident == nil || nw.near[d-ownerUp] != noPlace && nw.ns.cands[nw.near[d-ownerUp]].rec == owner
}

// sameInstance reports rec, met by the walk, as owner's instance: owner or a copy holding its fields (EVALUATION.md §4.2).
func sameInstance(rec, owner *value.Record) bool {
	return rec == owner || slices.Equal(rec.Fields, owner.Fields)
}

// ownedBy reports owner, met at depth i, as inst: inst or a copy holding its fields, or an amended
// copy of it, whose untouched entries keep inst's identity, as the place's history or the value its
// origin produced holds (DECISIONS 478, 486, 492).
func (nw *nodeWalk) ownedBy(owner, inst *value.Record, i int) bool {
	if sameInstance(owner, inst) {
		return true
	}
	a := nw.ns.s.a
	return slices.ContainsFunc(a.History(nw.vals[:i+1]...), func(v value.Value) bool {
		rec, isRec := a.Produced(v).(*value.Record)
		return isRec && sameInstance(rec, inst)
	})
}

// owns reports the walk's let owning c: the let c is named after, or for a load.defines table
// no ref names, the let whose initializer is the load (API.md R11).
func (nw *nodeWalk) owns(c *types.Collection) bool {
	if c.Name != "" {
		return c.Pkg == nw.r.pkg.Path && c.Name == nw.r.obj.Name()
	}
	_, isLoad := syntax.Unparen(initializer(nw.r.obj.Decl())).(*syntax.LoadExpr)
	return isLoad
}

// candidate adds the entry or element at the walk's depth, in its trie position, and is its index.
func (nw *nodeWalk) candidate(rec *value.Record, c typedChild, ct types.Type, under int) int {
	d := len(nw.segs)
	k := d
	for nw.pos[k] == noPlace {
		k--
	}
	for ; k < d; k++ {
		nw.pos[k+1] = nw.ns.trie.add(nw.pos[k], nw.segs[k])
	}
	node := GraphNode{
		ID: Path{Package: nw.r.pkg.Path, Root: nw.r.obj.Name(), Segs: nw.segs}.String(), Kind: c.kind,
		Package: nw.r.pkg.Path, Type: elemType(ct), Retired: c.kind == NodeEntry && rec.Ident.Retired,
	}
	if p := rec.P; p != nil {
		node.Span = p.Span
	}
	ident := nodeIdent{coll: rec.Ident.Coll, owner: rec.Ident.Owner, key: rec.Ident.Key}
	owning := d == ownerUp && rec.Ident.Owner != nil && !nw.aliased
	row := nodeRow{GraphNode: node, trie: nw.pos[d], ident: ident}
	nw.ns.cands = append(nw.ns.cands, candidate{row: row, rec: rec, under: under, owning: owning, aliased: nw.aliased})
	nw.ns.byCand[ident] = append(nw.ns.byCand[ident], len(nw.ns.cands)-1)
	return len(nw.ns.cands) - 1
}

// readsLet reports r holding another let's value: its initializer names a let, read through
// fields, keys or indices, as `let aw: Pl = z` does (API.md R11: a let holding a shared record is
// no node).
func readsLet(info *check.Info, r rootRef) bool {
	for e := syntax.Unparen(initializer(r.obj.Decl())); ; {
		switch x := e.(type) {
		case *syntax.IdentExpr:
			return isLet(info.Uses[x])
		case *syntax.SelectorExpr:
			if isLet(info.NameUses[x.Name]) {
				return true
			}
			e = syntax.Unparen(x.X)
		case *syntax.IndexExpr:
			e = syntax.Unparen(x.X)
		default:
			return false
		}
	}
}

func isLet(obj check.Object) bool {
	return obj != nil && obj.Kind() == check.ObjLet
}
