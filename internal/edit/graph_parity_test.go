package edit_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
)

// parity is what a parity check covered: the targets it compared and their edges.
type parity struct{ targets, edges int }

// checkParity is API.md R11, R12 and R14 on g: every node resolves to its ID, listed once; each
// edge's From is its Path's longest node prefix; the edges to each target are, in order, the value
// and key refs Refs finds for it, path, kind, span and computedness alike.
func checkParity(t *testing.T, f fixture, g *edit.Graph) parity {
	t.Helper()
	ids := map[string]int{}
	for i, n := range g.Nodes {
		if _, dup := ids[n.ID]; dup {
			t.Errorf("R11: node %s listed twice", n.ID)
		}
		ids[n.ID] = i
		checkNode(t, f, n)
	}
	byTo := map[int][]edit.GraphEdge{}
	for _, e := range g.Edges {
		byTo[e.To] = append(byTo[e.To], e)
		if want := longestNode(ids, e.Path); want != e.From {
			t.Errorf("R12: %s is owned by %s, want %s", e.Path, g.Nodes[e.From].ID, g.Nodes[want].ID)
		}
	}
	checkComplete(t, f, g)
	var out parity
	for i, n := range g.Nodes {
		if n.Kind == edit.NodeRoot {
			continue
		}
		out.edges += checkRefsOf(t, f, n.ID, byTo[i])
		out.targets++
	}
	return out
}

// checkComplete is R11's node set: every identity Refs accepts as a target, met by a walk of
// the lets, is exactly one node, and every entry or element node is such an identity.
func checkComplete(t *testing.T, f fixture, g *edit.Graph) {
	t.Helper()
	nodes := map[any]string{}
	for _, n := range g.Nodes {
		if n.Kind != edit.NodeEntry && n.Kind != edit.NodeElement {
			continue
		}
		k, ok := edit.IdentityAt(f.Snapshot, n.ID)
		if other, dup := nodes[k]; !ok || dup {
			t.Errorf("R11: node %s is no target, or one identity with %s", n.ID, other)
		}
		nodes[k] = n.ID
	}
	var missing []string
	for k, path := range edit.Targets(f.Snapshot) { //canon:unordered sorted below
		if _, ok := nodes[k]; !ok {
			missing = append(missing, path)
		}
	}
	slices.Sort(missing)
	if len(missing) > 0 {
		t.Errorf("R11: no node for the targets %v", missing)
	}
}

// checkNode is a node's ID resolving to itself, with its type: the one Value reads (§5.6 Type).
func checkNode(t *testing.T, f fixture, n edit.GraphNode) {
	t.Helper()
	r := resolveOrNone(t, f, n)
	if r == nil {
		return
	}
	typ, err := f.Type(*r)
	if err != nil || r.Canonical != n.ID || fmt.Sprint(typ) != fmt.Sprint(n.Type) {
		t.Errorf("R11: node %s typed %v resolves to %s typed %v (%v)", n.ID, n.Type, r.Canonical, typ, err)
	}
}

// resolveOrNone is Resolve of a node's ID; nil for a root without a value, which keeps its type.
func resolveOrNone(t *testing.T, f fixture, n edit.GraphNode) *edit.Resolved {
	t.Helper()
	p, err := edit.Parse(n.ID)
	if err != nil {
		t.Fatalf("R11: node %s: %v", n.ID, err)
	}
	r, err := edit.Resolve(f.Snapshot, p)
	switch {
	case errors.Is(err, edit.ErrNoValue) && n.Kind == edit.NodeRoot:
		return nil
	case err != nil:
		t.Fatalf("R11: node %s: %v", n.ID, err)
	}
	return &r
}

// checkRefsOf compares the edges to one node with Refs' value and key refs of it (R11, R14),
// and is how many there are.
func checkRefsOf(t *testing.T, f fixture, id string, edges []edit.GraphEdge) int {
	t.Helper()
	r := resolve(t, f, id)
	refs, computed, err := edit.ValueRefs(context.Background(), f.Snapshot, r)
	if err != nil {
		t.Fatalf("ValueRefs(%s): %v", id, err)
	}
	all, err := f.Refs(context.Background(), r)
	if err != nil {
		t.Fatalf("Refs(%s): %v", id, err)
	}
	listed := slices.DeleteFunc(all, func(r edit.Ref) bool { return r.Kind != edit.RefValue && r.Kind != edit.RefKey })
	if !slices.Equal(listed, refs) {
		t.Fatalf("ValueRefs(%s) differs from Refs: %v, want %v", id, refs, listed)
	}
	if len(edges) != len(refs) {
		t.Errorf("R11: %d edges to %s, want %d: %v, want %v", len(edges), id, len(refs), edges, refs)
		return len(refs)
	}
	for j, e := range edges {
		r := refs[j]
		if e.Path != r.Package+":"+r.Path || e.Package != r.Package || e.Kind != r.Kind || e.Span != r.Span || e.Computed != computed[j] {
			t.Errorf("R14: edge %d to %s is %+v, want %+v (computed %v)", j, id, e, r, computed[j])
		}
	}
	return len(refs)
}

// longestNode is the node whose ID is the longest prefix of path by segments (R12).
func longestNode(ids map[string]int, path string) int {
	p, err := edit.Parse(path)
	if err != nil {
		return -1
	}
	for n := len(p.Segs); n >= 0; n-- {
		if i, ok := ids[edit.Path{Package: p.Package, Root: p.Root, Segs: p.Segs[:n]}.String()]; ok {
			return i
		}
	}
	return -1
}

// API.md R11, R12, R14: on the examples project, with no layer and with every layer, every
// entry, element and member node's edges are its value and key refs (about 750 targets).
func TestGraphParityExamples(t *testing.T) {
	dir, roots := exampleRoots(t)
	for _, layers := range [][]string{nil, {"knights", "louis", "staging"}} {
		p, err := build.Open(project.OS(), dir, build.Options{Roots: roots, Layers: layers})
		if err != nil {
			t.Fatal(err)
		}
		f := analyze(t, p, nil, false)
		g, err := f.Graph(context.Background())
		if err != nil {
			t.Fatalf("layers %v: %v", layers, err)
		}
		got := checkParity(t, f, g)
		if got.targets < len(g.Nodes)/2 || got.edges != len(g.Edges) {
			t.Errorf("layers %v: %d targets, %d edges of %d nodes, %d edges", layers, got.targets, got.edges, len(g.Nodes), len(g.Edges))
		}
		t.Logf("layers %s: %d nodes, %d targets, %d edges, %d patterns", strings.Join(layers, ","), len(g.Nodes), got.targets, got.edges, len(g.Patterns))
	}
}

// API.md R11, R14: parity on the synthetic projects, a layer's amendments included (R7's
// merge by span), and the copies and let paths of graphG.
func TestGraphParityLaws(t *testing.T) {
	for _, f := range []fixture{refsFixture(t), lawFixture(t, []string{"dev"}, "a"), graphFixture(t, "", nil)} {
		g, err := f.Graph(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got := checkParity(t, f, g); got.edges != len(g.Edges) {
			t.Errorf("%d edges, %d of them to targets", len(g.Edges), got.edges)
		}
	}
}
