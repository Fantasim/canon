package canon_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// graphOf is every item Graph passes, nodes and edges apart.
func graphOf(t *testing.T, p *canon.Project) ([]*canon.GraphNode, []*canon.GraphEdge) {
	t.Helper()
	var nodes []*canon.GraphNode
	var edges []*canon.GraphEdge
	err := p.Graph(context.Background(), func(it canon.GraphItem) error {
		switch {
		case (it.Node == nil) == (it.Edge == nil):
			return fmt.Errorf("item %+v holds not exactly one of Node and Edge", it)
		case it.Node != nil:
			if len(edges) > 0 {
				return fmt.Errorf("node %s after an edge", it.Node.ID)
			}
			nodes = append(nodes, it.Node)
		default:
			edges = append(edges, it.Edge)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return nodes, edges
}

// API.md R11, R14, P10, GraphNode.Type: on the examples, every eighth node reads back through
// Value at its ID with its type, and every fourth target's edges (about 190 of 750) are Refs'
// value and key refs; internal/edit's TestGraphParityExamples checks them all.
func TestGraphParityAPI(t *testing.T) {
	p, _ := openViewExamples(t)
	nodes, edges := graphOf(t, p)
	byTo := map[string][]*canon.GraphEdge{}
	for _, e := range edges {
		byTo[e.To] = append(byTo[e.To], e)
	}
	targets := 0
	for i, n := range nodes {
		if i%8 == 0 {
			checkNodeValue(t, p, n)
		}
		if n.Kind == canon.NodeRoot {
			continue
		}
		if targets++; targets%4 == 1 {
			checkEdgesTo(t, p, n.ID, byTo[n.ID])
		}
	}
}

// checkNodeValue is Value at a node's ID: the same path, its Type (a root without a value aside).
func checkNodeValue(t *testing.T, p *canon.Project, n *canon.GraphNode) {
	t.Helper()
	v, err := p.Value(context.Background(), n.ID)
	switch {
	case errors.Is(err, canon.ErrNoValue) && n.Kind == canon.NodeRoot:
	case err != nil:
		t.Errorf("R11: Value(%s): %v", n.ID, err)
	case v.Path != n.ID || v.Type.Expr != n.Type:
		t.Errorf("R11: node %s typed %s is %s typed %s", n.ID, n.Type, v.Path, v.Type.Expr)
	}
}

// checkEdgesTo compares the edges to id with Refs(id)'s value and key refs, in order (R11, R14).
func checkEdgesTo(t *testing.T, p *canon.Project, id string, edges []*canon.GraphEdge) {
	t.Helper()
	res, err := p.Refs(context.Background(), id)
	if err != nil {
		t.Fatalf("Refs(%s): %v", id, err)
	}
	refs := slices.DeleteFunc(res.Refs, func(r canon.Ref) bool { return r.Kind != canon.RefValue && r.Kind != canon.RefKey })
	if len(refs) != len(edges) {
		t.Errorf("R11: %d edges to %s, want %d", len(edges), id, len(refs))
		return
	}
	for i, r := range refs {
		if e := edges[i]; e.Path != r.Package+":"+r.Path || e.Kind != r.Kind || e.Span != r.Span {
			t.Errorf("R14: edge %d to %s is %+v, want %+v", i, id, e, r)
		}
	}
}

// API.md §5.6: refLaw's graph, every path with its package (P10), patterns (R13) and owners (R12).
func TestGraphRefLaw(t *testing.T) {
	p, _ := openLaw(t, refLaw)
	nodes, edges := graphOf(t, p)
	var got []string
	for _, n := range nodes {
		got = append(got, fmt.Sprintf("%s %s %s %s:%d", n.Kind, n.ID, n.Type, n.File, n.Line))
	}
	for _, e := range edges {
		got = append(got, fmt.Sprintf("%s %s %s %s %s %v %s:%d", e.Kind, e.From, e.Path, e.Pattern, e.To, e.Computed, e.File, e.Line))
	}
	want := []string{
		"member a:Mood.calm a.Mood a/a.canon:5", "member a:Mood.angry a.Mood a/a.canon:5",
		"root a:statuses table a.Status a/a.canon:16", "entry a:statuses.open a.Status a/a.canon:17",
		"entry a:statuses.done a.Status a/a.canon:18", "root a:first ref a.statuses a/a.canon:22",
		"root b:board b.Board b/b.canon:15",
		"value a:statuses.open a:statuses.open.mood a:statuses[*].mood a:Mood.calm false a/a.canon:17",
		"value a:statuses.done a:statuses.done.mood a:statuses[*].mood a:Mood.angry false a/a.canon:18",
		"value a:first a:first a:first a:statuses.open false a/a.canon:22",
		"value b:board b:board.start b:board.start a:statuses.open false b/b.canon:15",
		"key b:board b:board.weights[done] b:board.weights[*] a:statuses.done false b/b.canon:15",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Graph =\n%q\nwant\n%q", got, want)
	}
}

// API.md R14, R11: two calls give the same sequence, and every item passed is new.
func TestGraphDeterministic(t *testing.T) {
	p, _ := openLaw(t, refLaw)
	n1, e1 := graphOf(t, p)
	n2, e2 := graphOf(t, p)
	same := slices.EqualFunc(n1, n2, func(a, b *canon.GraphNode) bool { return *a == *b && a != b }) &&
		slices.EqualFunc(e1, e2, func(a, b *canon.GraphEdge) bool { return *a == *b && a != b })
	if !same {
		t.Errorf("R14: two Graphs differ, or share items:\n%v %v\n%v %v", n1, e1, n2, e2)
	}
}

// API.md R15: a let that could hold a ref but has no value fails Graph before fn is called,
// with Refs' error for it; an error fn returns stops Graph, returned unchanged; so does ctx.
func TestGraphErrors(t *testing.T) {
	law := map[string]string{"bad/bad.canon": "/// Bad.\npackage bad\n\nimport a\n\n/// A pool.\nlet pool: [ref a.statuses] = [a.statuses.open]\n\n/// Broken.\nlet broken: ref a.statuses = pool[5]\n"}
	maps.Copy(law, refLaw)
	bad, _ := openLaw(t, law)
	calls := 0
	err := bad.Graph(context.Background(), func(canon.GraphItem) error { calls++; return nil })
	var pe *canon.PathError
	if !errors.Is(err, canon.ErrNoValue) || !errors.As(err, &pe) || pe.Path != "bad:broken" || pe.Detail != "root bad:broken" || len(pe.Findings) == 0 || calls > 0 {
		t.Errorf("R15: Graph with bad:broken poisoned = %v after %d items", err, calls)
	}
	p, _ := openLaw(t, refLaw)
	stop := errors.New("enough")
	calls = 0
	err = p.Graph(context.Background(), func(canon.GraphItem) error {
		if calls++; calls == 3 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || errors.Unwrap(err) != nil || calls != 3 { // unchanged: not even wrapped
		t.Errorf("R15: Graph after fn's error = %v after %d items, want it after 3", err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	err = p.Graph(ctx, func(canon.GraphItem) error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Graph cancelled while streaming = %v, want context.Canceled", err)
	}
}

// API.md R15: a panic in fn is the caller's own, never the compiler's *InternalError.
func TestGraphPanicInFnIsTheCallers(t *testing.T) {
	p, _ := openLaw(t, refLaw)
	defer func() {
		if r := recover(); r != "caller" {
			t.Errorf("Graph with a panicking fn recovered %v, want the caller's panic", r)
		}
	}()
	_ = p.Graph(context.Background(), func(canon.GraphItem) error { panic("caller") })
	t.Error("Graph returned after fn panicked")
}
