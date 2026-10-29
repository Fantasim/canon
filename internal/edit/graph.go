package edit

import (
	"cmp"
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// NodeKind is what a node of the reference graph is (API.md R11).
type NodeKind uint8

// GraphNode is a node of the reference graph (API.md R11): ID is its canonical path with its
// package (P10); Type, nil for none, is the type at its location (P9).
type GraphNode struct {
	ID      string
	Kind    NodeKind
	Package string
	Type    types.Type
	Retired bool
	Span    source.Span
}

// GraphEdge is a value or key reference (API.md R11-R13): From and To index the graph's Nodes,
// Pattern its Patterns; Path is canonical with its package.
type GraphEdge struct {
	Path     string
	Package  string
	Span     source.Span
	From, To int
	Pattern  int
	Kind     RefKind
	Computed bool
}

// Graph is every node and every edge of a snapshot, each in R14 order.
type Graph struct {
	Nodes    []GraphNode
	Edges    []GraphEdge
	Patterns []string
}

// Graph is s's reference graph from one walk of its lets (API.md R11-R15). A let that could hold
// a reference, an entry or an element but whose value or a key in it is missing, or one typed
// Error, is its error: the first such let in (package, name) order, wrapping ErrNoValue.
func (s *Snapshot) Graph(ctx context.Context) (*Graph, error) {
	ns, err := s.graphNodes(ctx)
	if err != nil {
		return nil, err
	}
	if ns.failed != nil {
		return nil, ns.failed
	}
	sc := s.newGraphScan(ns)
	if err := sc.w.run(ctx, s); err != nil {
		return nil, err
	}
	g := &Graph{Nodes: ns.nodes, Edges: sc.edges, Patterns: sc.pats.text}
	slices.SortStableFunc(g.Edges, func(a, b GraphEdge) int {
		return cmp.Or(s.refOrder(a.ref(), b.ref()), cmp.Compare(g.Nodes[a.To].ID, g.Nodes[b.To].ID))
	})
	return g, nil
}

// ref is the edge as Refs lists it, its path without the package, for R8's order (API.md R14).
func (e GraphEdge) ref() Ref {
	return Ref{Kind: e.Kind, Package: e.Package, Path: e.Path[len(e.Package)+len(packageMark):], Span: e.Span}
}

// nodeOrder is R14's order of nodes: their spans as R8 orders them, then their IDs.
func (s *Snapshot) nodeOrder(a, b GraphNode) int {
	return cmp.Or(
		cmp.Compare(s.display(a.Span.File), s.display(b.Span.File)),
		cmp.Compare(a.Span.Start, b.Span.Start), cmp.Compare(a.Span.End, b.Span.End),
		cmp.Compare(a.ID, b.ID),
	)
}
