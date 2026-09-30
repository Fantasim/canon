package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/wire"
)

// atStep is one step of a load's `at:` path (WIRE.md 6.3, LOD-03): a `*` reads a map or list
// level from the object or array it stands on.
type atStep = wire.AtStep

// starSteps are the steps of load's `at:` path when a `*` there reads items that state their
// value deeper than the item itself, nil otherwise (WIRE.md 6.3).
func starSteps(load *syntax.LoadExpr) []atStep {
	steps := atSteps(load)
	first := slices.IndexFunc(steps, isStar)
	if first < 0 || !slices.ContainsFunc(steps[first:], func(s atStep) bool { return !isStar(s) }) {
		return nil
	}
	return steps
}

// isStar reports a `*` step.
func isStar(s atStep) bool { return s.Kind == wire.AtStar }

// markStars records in out the containers the `*`s of steps read under n.
func markStars(out map[string][]atStep, n *jsonsrc.Node, steps []atStep) {
	for n != nil && len(steps) > 0 && !isStar(steps[0]) {
		n, steps = atChild(n, steps[0]), steps[1:]
	}
	if n == nil || len(steps) == 0 {
		return
	}
	out[n.Pointer()] = steps[1:]
	for _, c := range itemNodes(n) {
		markStars(out, c, steps[1:])
	}
}

// itemNodes are an object's member values or an array's elements.
func itemNodes(n *jsonsrc.Node) []*jsonsrc.Node {
	if n.Kind == jsonsrc.Array {
		return n.Elems
	}
	out := make([]*jsonsrc.Node, len(n.Members))
	for i, m := range n.Members {
		out[i] = m.Value
	}
	return out
}

// atChild is the member or element a name or index step selects in n, nil when there is none.
func atChild(n *jsonsrc.Node, st atStep) *jsonsrc.Node {
	switch {
	case st.Kind == wire.AtName && n.Kind == jsonsrc.Object:
		if i := objMember(n, st.Name); i >= 0 {
			return n.Members[i].Value
		}
	case st.Kind == wire.AtIndex && n.Kind == jsonsrc.Array && st.Index < len(n.Elems):
		return n.Elems[st.Index]
	}
	return nil
}

// restOf are the steps from an item of a `*` level to the value it states: those before the
// next `*`.
func restOf(after []atStep) []atStep {
	if i := slices.IndexFunc(after, isStar); i >= 0 {
		return after[:i]
	}
	return after
}

// descend is the node rest leads to from item, nil when the item lacks it.
func descend(item *jsonsrc.Node, rest []atStep) *jsonsrc.Node {
	for _, st := range rest {
		if item == nil {
			return nil
		}
		item = atChild(item, st)
	}
	return item
}

// wrapRest is the item that states n at the end of rest: an object per name; an index only
// as the first element of an array, since no item states a later element alone (API.md E2).
func wrapRest(rest []atStep, n *jsonsrc.Node) (*jsonsrc.Node, error) {
	for _, st := range slices.Backward(rest) {
		switch {
		case st.Kind != wire.AtIndex:
			n = &jsonsrc.Node{Kind: jsonsrc.Object, Members: []jsonsrc.Member{{Key: st.Name, Value: n}}}
		case st.Index == 0:
			n = &jsonsrc.Node{Kind: jsonsrc.Array, Elems: []*jsonsrc.Node{n}}
		default:
			return nil, ErrBadOp
		}
	}
	return n, nil
}

// atSteps is the `at:` path of load, parsed by WIRE.md 6.3's grammar; nil when it has none or,
// which the load reports, a malformed one.
func atSteps(load *syntax.LoadExpr) []atStep {
	text, ok := atText(load)
	if !ok {
		return nil
	}
	steps, _ := wire.ParseAt(text)
	return steps
}

// atText is the text of load's `at:` option, false when it has none written as plain text.
func atText(load *syntax.LoadExpr) (string, bool) {
	if load == nil {
		return "", false
	}
	for _, a := range load.Args {
		lit, ok := a.Value.(*syntax.StringLit)
		if a.Name != nil && a.Name.Name == loadAtOption && ok && len(lit.Parts) == 1 && lit.Parts[0].Interp == nil {
			return lit.Parts[0].Text, true
		}
	}
	return "", false
}
