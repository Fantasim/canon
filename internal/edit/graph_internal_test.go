package edit

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/value"
)

// graphTree is a project of n items, each with refs to two others and a keyed list of slots
// referring to a third: about 3n value edges and 2n element nodes.
func graphTree(n int) rowsFS {
	var b strings.Builder
	b.WriteString("/// G.\npackage g\n\n/// A slot.\nrecord Slot {\n  /// Its number.\n  n: Int\n  /// Its target.\n  to: ref items\n}\n\n")
	b.WriteString("/// An item.\nrecord Item {\n  /// Its links.\n  next: [ref items] = []\n  /// Its slots.\n  slots: [Slot] keyed by n = []\n}\n\n")
	b.WriteString("/// The items.\nlet items: table Item = {\n")
	for i := range n {
		fmt.Fprintf(&b, "  i%d { next: [i%d, i%d], slots: [{ n: 1, to: i%d }, { n: 2, to: i%d }] }\n", i, (i+1)%n, (i+2)%n, (i+3)%n, i)
	}
	b.WriteString("}\n")
	return rowsFS{
		"law/project.canon": {Data: []byte("project acme {\n  canon: \"0.1\"\n}\n")},
		"law/g/g.canon":     {Data: []byte(b.String())},
	}
}

// graphSnapshot is the snapshot of tree, every package analyzed.
func graphSnapshot(tb testing.TB, tree rowsFS) *Snapshot {
	tb.Helper()
	p, err := build.Open(tree, "/law", build.Options{})
	if err != nil {
		tb.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		tb.Fatal(err)
	}
	return NewSnapshot(a)
}

// API.md R15: a let holding a keyed-list element whose key is not known fails the graph,
// named like a let without a value. No source makes one (an input key poisons its let), so
// the key is cleared in the settled value.
func TestGraphFailsOnUnknownKey(t *testing.T) {
	s := graphSnapshot(t, graphTree(2))
	items, err := s.force(s.rootNamed("g", "items"))
	if err != nil {
		t.Fatal(err)
	}
	slots := items.(*value.Table).Entries[0].Fields[1].(*value.List)
	slots.Elems[0].(*value.Record).Fields[0] = nil
	_, err = s.Graph(context.Background())
	var pe *PathError
	if !errors.As(err, &pe) || !errors.Is(err, ErrNoValue) || pe.Root.Name != "items" {
		t.Errorf("Graph with a key unknown = %v, want ErrNoValue naming g:items", err)
	}
}

// rootNamed is the let name of package pkg.
func (s *Snapshot) rootNamed(pkg, name string) rootRef {
	for _, obj := range s.byPath[pkg].Decls {
		if obj.Name() == name {
			return rootRef{pkg: s.byPath[pkg], obj: obj}
		}
	}
	return rootRef{}
}

// BenchmarkGraph is Graph on 20 000 items (80 000 edges, 60 000 nodes): its time, and the heap
// it uses at its peak and holds at the end, per edge.
func BenchmarkGraph(b *testing.B) {
	const items = 20_000
	s := graphSnapshot(b, graphTree(items))
	var before, after runtime.MemStats
	b.ResetTimer()
	for range b.N {
		runtime.GC()
		runtime.ReadMemStats(&before)
		peak := before.HeapInuse
		stop := sampleHeap(&peak)
		g, err := s.Graph(context.Background())
		stop()
		if err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(peak-before.HeapInuse)/float64(len(g.Edges)), "peakB/edge")
		runtime.GC()
		runtime.ReadMemStats(&after)
		b.ReportMetric(float64(after.HeapInuse-before.HeapInuse)/float64(len(g.Edges)), "B/edge")
		b.ReportMetric(float64(len(g.Edges)), "edges")
		runtime.KeepAlive(g)
	}
}
