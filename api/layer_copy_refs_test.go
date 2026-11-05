package canon_test

import (
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// copyRefsSrc is p: pi is a plain copy of b1's inner record at depth 0, hb holds one at depth 1 and
// sb spreads one; g* probes whether m's keys and subs' picks equal the x the copy holds, whether
// that x is still b1's, and whether the picks dereference to its content.
const copyRefsSrc = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\n" +
	"record P {\n  pick: ref S\n}\n\n" +
	"record I {\n  id: String\n  oo: [S] keyed by name\n  m: {ref S: Int} = { x: 1 }\n  subs: [P] = []\n}\n\n" +
	"record B {\n  id: String\n  inner: I\n}\n\n" +
	"let b1: B = { id: \"b1\", inner: { id: \"i\", oo: [{ name: \"x\" }, { name: \"y\" }] } }\n\n" +
	"let pi: I = b1.inner\n\n" +
	"let hb: B = { id: \"hb\", inner: b1.inner }\n\n" +
	"let sb: I = { ...b1.inner, id: \"sb\" }\n\n" +
	"let gp: [Bool] = [pi.m.keys().contains(pi.oo.x), [s.pick for s in pi.subs].contains(pi.oo.x), pi.oo.x == b1.inner.oo.x, [s.pick.w for s in pi.subs] == [pi.oo.x.w]]\n\n" +
	"let gh: [Bool] = [hb.inner.m.keys().contains(hb.inner.oo.x), [s.pick for s in hb.inner.subs].contains(hb.inner.oo.x), hb.inner.oo.x == b1.inner.oo.x, [s.pick.w for s in hb.inner.subs] == [hb.inner.oo.x.w]]\n\n" +
	"let gs: [Bool] = [sb.m.keys().contains(sb.oo.x), [s.pick for s in sb.subs].contains(sb.oo.x), sb.oo.x == b1.inner.oo.x, [s.pick.w for s in sb.subs] == [sb.oo.x.w]]\n"

// elementNodes are the ids of p's element nodes in p's graph.
func elementNodes(t *testing.T, p *canon.Project) []string {
	t.Helper()
	nodes, _ := graphOf(t, p)
	var out []string
	for _, n := range nodes {
		if n.Package == "p" && n.Kind == canon.NodeElement {
			out = append(out, n.ID)
		}
	}
	return out
}

// copyRefsCase is a block of layer lines amending root, whose probe let is probe: what it gives
// active, and whether a node of the copy's own x appears.
type copyRefsCase struct {
	root, probe, at string
	lines           []string
	want            string
	own             bool
}

// DECISIONS 410, 486: a ref or map key a layer writes into a copy names the entry the copy's field
// holds, by value: the source's while unchanged, the copy's own once a line amends it, in either
// order; with the layer inactive the probes are the copy's base ones.
func TestLayerCopyWrittenRefs(t *testing.T) {
	writes := func(at string) []string { return []string{at + "m: { x: 3 }", at + `subs: [{ pick: "x" }]`} }
	var cases []copyRefsCase
	for _, c := range []struct{ root, probe, at string }{{"pi", "p:gp", ""}, {"hb", "p:gh", "inner."}, {"sb", "p:gs", ""}} {
		own := c.at + "oo.x.w: 5"
		cases = append(cases,
			copyRefsCase{c.root, c.probe, c.at, writes(c.at), "[true, true, true, true]", false},
			copyRefsCase{c.root, c.probe, c.at, append(writes(c.at), own), "[true, true, false, true]", true},              // regression guard: passes on the base
			copyRefsCase{c.root, c.probe, c.at, append([]string{own}, writes(c.at)...), "[true, true, false, true]", true}, // regression guard: passes on the base
		)
	}
	for _, c := range cases {
		t.Run(c.root+" "+strings.Join(c.lines, "; "), func(t *testing.T) {
			body := "amend " + c.root + " {\n  " + strings.Join(c.lines, "\n  ") + "\n}\n"
			off, on := openLayered(t, copyRefsSrc, body, false), openLayered(t, copyRefsSrc, body, true)
			checkValues(t, on, "DECISIONS 410", map[string]string{c.probe: c.want})
			checkValues(t, off, "DECISIONS 410", map[string]string{c.probe: "[true, false, true, false]"})
			nodes0, nodes1 := elementNodes(t, off), elementNodes(t, on)
			node := "p:" + c.root + "." + c.at + "oo[x]"
			if got := slices.Contains(nodes1, node); got != c.own || slices.Contains(nodes0, node) {
				t.Errorf("DECISIONS 478: node %s with the layer %v, want %v; nodes\n%s", node, got, c.own, strings.Join(nodes1, "\n"))
			}
		})
	}
}
