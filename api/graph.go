package canon

import (
	"context"
	"errors"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
)

// GraphItem is one item Graph streams: exactly one of Node and Edge is set (API.md §5.6).
type GraphItem struct {
	Node *GraphNode
	Edge *GraphEdge
}

// GraphNode is a root, a table entry, a keyed-list element or an enum member (rule R11).
type GraphNode struct {
	ID      string // canonical path with its package (rule P10)
	Kind    NodeKind
	Package string
	Type    string // Value(ID).Type.Expr; a member's enum, qualified
	Retired bool   // a retired table entry or enum member
	Span           // root: the let's name; member: its name; entry, element: its record's provenance
}

// GraphEdge is a value or key reference to a node (rules R11-R13); every path has its package.
type GraphEdge struct {
	From     string // the owning node's ID (rule R12)
	Path     string // the referring value or map entry
	Pattern  string // rule R13
	To       string // the target node's ID
	Kind     RefKind
	Computed bool // code computed the value (rule R7)
	Span
}

// Graph calls fn with every node, then every edge, of the project's reference graph, from the
// analysis Refs reads, once (rules R11-R15); a let R15 names fails it before fn is called, and
// an error fn returns stops it, returned as is (a panic in fn is fn's own).
func (p *Project) Graph(ctx context.Context, fn func(GraphItem) error) error {
	g, files, err := p.graph(ctx)
	if err != nil {
		return err
	}
	return streamGraph(ctx, files, g, fn)
}

// graph is the whole graph Graph streams, computed before any item is (rule R15).
func (p *Project) graph(ctx context.Context) (g *edit.Graph, files diag.Files, err error) {
	defer recoverInternal(&err)
	ws, err := p.read(ctx)
	if err != nil {
		return nil, nil, err
	}
	a, err := analyze(ctx, ws, nil)
	if err != nil {
		return nil, nil, err
	}
	s := &snapshot{a: a, s: edit.NewSnapshot(a), ctx: ctx}
	if g, err = s.s.Graph(ctx); err != nil {
		return nil, nil, s.graphError(ctx, err)
	}
	if _, err := p.revision(ctx, ws); err != nil {
		return nil, nil, err
	}
	return g, a.Files(), nil
}

// graphError is edit's refusal of Graph as the API's: Refs' error for the let it names (R15).
func (s *snapshot) graphError(ctx context.Context, err error) error {
	var pe *edit.PathError
	if !errors.As(err, &pe) {
		return s.refsError(ctx, "", err)
	}
	return s.refsError(ctx, edit.Path{Package: pe.Root.Pkg, Root: pe.Root.Name}.String(), err)
}

// streamGraph passes each node, then each edge, of g to fn, every item new (rule R11).
func streamGraph(ctx context.Context, files diag.Files, g *edit.Graph, fn func(GraphItem) error) error {
	for _, n := range g.Nodes {
		node := &GraphNode{ID: n.ID, Kind: nodeKinds[n.Kind], Package: n.Package, Retired: n.Retired, Span: locate(files, n.Span)}
		if n.Type != nil {
			node.Type = n.Type.String()
		}
		if err := yieldItem(ctx, fn, GraphItem{Node: node}); err != nil {
			return err
		}
	}
	for _, e := range g.Edges {
		edge := &GraphEdge{
			From: g.Nodes[e.From].ID, Path: e.Path, Pattern: g.Patterns[e.Pattern], To: g.Nodes[e.To].ID,
			Kind: refKinds[e.Kind], Computed: e.Computed, Span: locate(files, e.Span),
		}
		if err := yieldItem(ctx, fn, GraphItem{Edge: edge}); err != nil {
			return err
		}
	}
	return nil
}

// yieldItem is fn of it, unless ctx is done.
func yieldItem(ctx context.Context, fn func(GraphItem) error, it GraphItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(it)
}
