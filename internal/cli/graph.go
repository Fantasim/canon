package cli

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	canon "github.com/fantasim/canonlang/api"
)

// runGraph is `canon graph` (CLI.md §3.18): the project's reference graph, nodes and edges.
func runGraph(inv *invocation) int {
	if len(inv.args) != 0 {
		return inv.fail(fmt.Errorf(fmtArgs, cmdGraph, errNoArgs))
	}
	p, err := inv.openProject()
	if err != nil {
		return inv.fail(err)
	}
	defer func() { _ = p.Close() }()
	if inv.opt.format == formatJSON {
		err = inv.graphJSON(p)
	} else {
		err = inv.graphText(p)
	}
	if err != nil {
		return inv.fail(err)
	}
	return exitOK
}

// graphKey is what a group of edges shares (CLI.md §3.18): pattern, kind and target type.
type graphKey struct {
	pattern string
	kind    canon.RefKind
	typ     string
}

// graphPair is one example edge: its source and target nodes.
type graphPair struct{ from, to string }

// graphGroup is a group's edge count and its first distinct (from, to) pairs.
type graphGroup struct {
	count int
	pairs []graphPair
}

// add counts an edge and keeps its pair when it is one of the first distinct ones.
func (g *graphGroup) add(from, to string) {
	g.count++
	pair := graphPair{from, to}
	if len(g.pairs) < graphExamples && !slices.Contains(g.pairs, pair) {
		g.pairs = append(g.pairs, pair)
	}
}

// graphText writes the grouped view; only counts and example pairs are kept, never every edge.
func (inv *invocation) graphText(p *canon.Project) error {
	types := map[string]string{}
	groups := map[graphKey]*graphGroup{}
	nodes, edges := 0, 0
	err := p.Graph(inv.ctx, func(it canon.GraphItem) error {
		if it.Node != nil {
			nodes++
			types[it.Node.ID] = it.Node.Type
			return nil
		}
		edges++
		k := edgeKey(it.Edge, types)
		g := groups[k]
		if g == nil {
			g = &graphGroup{}
			groups[k] = g
		}
		g.add(it.Edge.From, it.Edge.To)
		return nil
	})
	if err != nil {
		return err
	}
	keys := slices.Collect(maps.Keys(groups))
	slices.SortFunc(keys, compareGraphKeys)
	w := inv.env.Stdout
	writeLine(w, fmt.Sprintf(fmtGraphHead, nodes, plural(nodes), edges, plural(edges), len(keys), plural(len(keys))))
	for _, k := range keys {
		g := groups[k]
		writeLine(w, outputIndent+fmt.Sprintf(fmtGraphGroup, g.count, k.kind, k.pattern, k.typ))
		for _, pr := range g.pairs {
			writeLine(w, graphExampleIndent+fmt.Sprintf(fmtGraphPair, pr.from, pr.to))
		}
	}
	return nil
}

// edgeKey is the group of e, its target's type read from the nodes seen so far.
func edgeKey(e *canon.GraphEdge, types map[string]string) graphKey {
	return graphKey{pattern: e.Pattern, kind: e.Kind, typ: types[e.To]}
}

// compareGraphKeys orders groups by pattern, kind, then type bytes.
func compareGraphKeys(a, b graphKey) int {
	return cmp.Or(cmp.Compare(a.pattern, b.pattern), cmp.Compare(a.kind, b.kind), cmp.Compare(a.typ, b.typ))
}

// plural is the plural ending of a noun counted n, empty when n is 1.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return pluralSuffix
}
