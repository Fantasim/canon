package cppgen

import (
	"fmt"
	"strconv"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// flatLevel is one container level of a nested literal laid out in arrays: its type, each
// container's bounds into the next level's items, and a map level's keys by item.
type flatLevel struct {
	t      ir.TypeRef
	bounds []int
	keys   []value.Value
}

// flatTree is a nested list or map literal as arrays: its container levels top down, then the leaves.
type flatTree struct {
	levels []flatLevel
	leaves []value.Value
	leafT  ir.TypeRef
}

// flatten lays v of type t out level by level; false for a shape arrays cannot hold: a keyed list,
// an optional container, a key or leaf arrayElem refuses, or no leaf at all.
func (g *gen) flatten(t ir.TypeRef, v value.Value) (flatTree, bool) {
	var tr flatTree
	items := []value.Value{v}
	for nestedContainer(t) {
		lvl := flatLevel{t: t, bounds: []int{0}}
		var next []value.Value
		for _, it := range items {
			children, keys, ok := containerItems(it)
			if !ok {
				return tr, false
			}
			next, lvl.keys = append(next, children...), append(lvl.keys, keys...)
			lvl.bounds = append(lvl.bounds, len(next))
		}
		if t.Kind == types.Map && !g.keysHeld(*t.Key, lvl.keys) {
			return tr, false
		}
		tr.levels, items, t = append(tr.levels, lvl), next, *t.Elem
	}
	_, leafOK := g.arrayElem(t)
	tr.leaves, tr.leafT = items, t
	return tr, leafOK && len(items) > 0
}

// keysHeld reports a map level's keys an array holds: some, of a type arrayElem takes.
func (g *gen) keysHeld(t ir.TypeRef, keys []value.Value) bool {
	_, ok := g.arrayElem(t)
	return ok && len(keys) > 0
}

// nestedContainer reports a list or map type whose elements are held in arrays level by level.
func nestedContainer(t ir.TypeRef) bool {
	switch t.Kind {
	case types.List:
		return t.KeyedBy == nil && t.Elem != nil
	case types.Map:
		return t.Key != nil && t.Elem != nil
	default:
		return false
	}
}

// containerItems are a list's elements or a map's values, and a map's keys.
func containerItems(v value.Value) (items, keys []value.Value, ok bool) {
	switch x := v.(type) {
	case *value.List:
		return x.Elems, nil, true
	case *value.Map:
		return x.Vals, x.Keys, len(x.Keys) == len(x.Vals)
	default:
		return nil, nil, false
	}
}

// hoistedNested is a literal nesting lists or maps built from arrays, the leaves in one, each
// level's bounds and keys in others, by one loop per level; false for a single level, which
// hoistedLiteral writes, too few elements, or a shape flatten refuses (DECISIONS 392).
func (g *gen) hoistedNested(t ir.TypeRef, v value.Value) (string, bool) {
	if g.pool == nil || !nestedContainer(t) || !nestedContainer(*t.Elem) {
		return "", false
	}
	tr, ok := g.flatten(t, v)
	if !ok || !g.hoists(ir.BulkCount(t, v)) { // the plan reserves its arrays' names by the same count
		return "", false
	}
	n := &nestedArrays{g: g, tr: tr}
	n.write()
	return n.build(0, strconv.Itoa(0), strconv.Itoa(tr.levels[0].bounds[1])), true
}

// nestedArrays are a flat tree's arrays once written: each level's bounds and keys, the leaves.
type nestedArrays struct {
	g            *gen
	tr           flatTree
	bounds, keys []string
	leaves       string
}

// write writes the tree's arrays: the bounds of each level below the top, each map level's keys, the leaves.
func (n *nestedArrays) write() {
	g := n.g
	n.bounds, n.keys = make([]string, len(n.tr.levels)), make([]string, len(n.tr.levels))
	for i, l := range n.tr.levels {
		if i > 0 {
			n.bounds[i] = g.pool.array(cppSize, intTexts(l.bounds))
		}
		if l.t.Kind == types.Map {
			elem, _ := g.arrayElem(*l.t.Key)
			n.keys[i] = g.pool.array(elem, g.arrayItems(*l.t.Key, l.keys))
		}
	}
	elem, _ := g.arrayElem(n.tr.leafT)
	n.leaves = g.pool.array(elem, g.arrayItems(n.tr.leafT, n.tr.leaves))
}

// arrayItems are vs, each of type t, as array elements.
func (g *gen) arrayItems(t ir.TypeRef, vs []value.Value) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = g.arrayText(func() string { return g.element(t, v) })
	}
	return out
}

// intTexts are ns as decimal literals.
func intTexts(ns []int) []string {
	out := make([]string, len(ns))
	for i, x := range ns {
		out[i] = strconv.Itoa(x)
	}
	return out
}

// build is the expression of level depth's container holding its next level's items lo to hi
// (exclusive): a list of leaves as a range of the leaves' array, else one loop filling it.
func (n *nestedArrays) build(depth int, lo, hi string) string {
	g, l := n.g, n.tr.levels[depth]
	storage, capture := g.storage(l.t), nestedInner
	if depth == 0 {
		capture = nestedOuter // a namespace-scope initializer's lambda captures nothing
	}
	idx := fmt.Sprintf(nestedIndexFormat, depth)
	last := depth == len(n.tr.levels)-1
	if l.t.Kind == types.List && last {
		return fmt.Sprintf(sliceRangeFormat, storage, n.leaves, lo, n.leaves, hi)
	}
	child := fmt.Sprintf(nestedLeafFormat, g.storage(n.tr.leafT), n.leaves, idx)
	if !last {
		next := n.bounds[depth+1]
		child = n.build(depth+1, fmt.Sprintf(indexFormat, next, idx), fmt.Sprintf(nextBoundFormat, next, idx))
	}
	if l.t.Kind == types.List {
		return fmt.Sprintf(nestedListFormat, capture, storage, lo, hi, idx, child)
	}
	key := fmt.Sprintf(nestedLeafFormat, g.storage(*l.t.Key), n.keys[depth], idx)
	return fmt.Sprintf(nestedMapFormat, capture, storage, lo, hi, idx, key, child)
}
