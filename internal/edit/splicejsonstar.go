package edit

import (
	"strings"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// jsonDiffAt is a diff of the JSON source at display that states the value at cursor k, knowing
// the containers whose items an `at:` path goes on past a `*` in (WIRE.md 6.3).
func (x *opCtx) jsonDiffAt(k int, display string) (*jsonDiff, error) {
	d := &jsonDiff{a: x.a}
	steps := starSteps(x.j.cur[min(k, len(x.j.cur)-1)].load)
	if steps == nil {
		return d, nil
	}
	root, err := x.a.jsonRoot(display)
	if err != nil {
		return nil, err
	}
	d.stars = map[string][]atStep{}
	markStars(d.stars, root, steps)
	return d, nil
}

// starOf is how the items of container n state their values: the `at:` steps after the `*`
// that reads n, false when no `*` reads it.
func (d *jsonDiff) starOf(n *jsonsrc.Node) ([]atStep, bool) {
	after, ok := d.stars[n.Pointer()]
	return after, ok
}

// starItem is the item a `*` level's container holds v with: v's source wire, its own `*`
// levels written as their steps lead, inside the steps to the next `*` (WIRE.md 6.3).
func (d *jsonDiff) starItem(v value.Value, f *types.Field, after []atStep) (*jsonsrc.Node, error) {
	rest := restOf(after)
	var node *jsonsrc.Node
	var err error
	if len(rest) < len(after) {
		node, err = d.starNode(v, f, after[len(rest)+1:])
	} else {
		node, err = d.a.wireNode(v, f)
	}
	if err != nil {
		return nil, err
	}
	return wrapRest(rest, node)
}

// starNode is collection v as the container a `*` reads it from, each item by starItem.
func (d *jsonDiff) starNode(v value.Value, f *types.Field, after []atStep) (*jsonsrc.Node, error) {
	switch x := v.(type) {
	case *value.Map:
		out := &jsonsrc.Node{Kind: jsonsrc.Object}
		for i, k := range x.Keys {
			key, err := wireKey(k)
			if err != nil {
				return nil, err
			}
			item, err := d.starItem(x.Vals[i], f, after)
			if err != nil {
				return nil, err
			}
			out.Members = append(out.Members, jsonsrc.Member{Key: key, Value: item})
		}
		return out, nil
	case *value.List:
		out := &jsonsrc.Node{Kind: jsonsrc.Array}
		for _, e := range x.Elems {
			item, err := d.starItem(e, f, after)
			if err != nil {
				return nil, err
			}
			out.Elems = append(out.Elems, item)
		}
		return out, nil
	}
	return d.a.wireNode(v, f)
}

// starSet writes nw over container n, which a `*` reads with after: every item removed, then
// every item of nw added, so that what the items hold beside their values is not rewritten flat.
func (d *jsonDiff) starSet(nw value.Value, n *jsonsrc.Node, f *types.Field, after []atStep) error {
	fresh, err := d.starNode(nw, f, after)
	if err != nil {
		return err
	}
	for _, m := range n.Members {
		d.remove(m.Value, int(m.KeySpan.Start))
	}
	for _, e := range n.Elems {
		d.remove(e, int(e.Span.Start))
	}
	for _, m := range fresh.Members {
		d.insert(n, 0, m.Key, m.Value) // at the first item, after its removal (applyOrder)
	}
	for _, e := range fresh.Elems {
		d.insert(n, 0, "", e)
	}
	return nil
}

// jsonItem is the member or element of the target's collection holding the target's node: the
// node itself, or under an `at:` path going on past a `*`, its ancestor at the `*`'s level.
func (x *opCtx) jsonItem() (*jsonsrc.Node, string, error) {
	n, display, err := x.jsonAt(x.res.Target)
	if err != nil {
		return nil, "", err
	}
	parent, _, ok := x.sibling()
	if !ok {
		return n, display, nil
	}
	ptr, deeper := x.itemPointer(parent, n, display)
	if !deeper {
		return n, display, nil
	}
	root, err := x.a.jsonRoot(display)
	if err != nil {
		return nil, "", err
	}
	if item := root.Find(ptr); item != nil {
		return item, display, nil
	}
	return nil, "", errNoMember
}

// itemPointer is the pointer of the child of the node stating parent that holds n, when n
// lies deeper than that child in the source at display.
func (x *opCtx) itemPointer(parent value.Value, n *jsonsrc.Node, display string) (string, bool) {
	c, cd, err := x.jsonAt(parent)
	if err != nil || cd != display {
		return "", false
	}
	rest, ok := strings.CutPrefix(n.Pointer(), c.Pointer()+pointerSep)
	tok, _, deeper := strings.Cut(rest, pointerSep)
	return c.Pointer() + pointerSep + tok, ok && deeper
}
