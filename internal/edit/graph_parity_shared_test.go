package edit_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// sharedParityHead is the project file of the shared parity fixtures.
const sharedParityHead = "project acme {\n  canon: \"0.1\"\n}\n"

// sharedParityL1 is a level-1 ref into each instance's own keyed list, each pick stated, and a
// layer replacing both picks.
var sharedParityL1 = mapFS{
	"law/project.canon": file(sharedParityHead),
	"law/p/p.canon": file("package p\n\nrecord S {\n  k: String\n}\n\nrecord A {\n  id: String\n  oo: [S] keyed by k\n  pick: ref S\n}\n\n" +
		"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\" }, { k: \"y\" }], pick: \"y\" },\n" +
		"  { id: \"second\", oo: [{ k: \"x\" }], pick: \"x\" },\n]\n"),
	"law/p/dev.layer.canon": file("package p\nlayer dev\n\namend aa {\n  second.pick: \"x\"\n  first.pick: \"x\"\n}\n"),
}

// sharedParityL2 is the same one level down, S's pick into its own inn, with level 1's too.
var sharedParityL2 = mapFS{
	"law/project.canon": file(sharedParityHead),
	"law/p/p.canon": file("package p\n\nrecord T {\n  k: String\n}\n\nrecord S {\n  k: String\n  inn: [T] keyed by k\n  pick: ref T\n}\n\n" +
		"record A {\n  id: String\n  oo: [S] keyed by k\n  pick: ref S\n}\n\n" +
		"let aa: [A] keyed by id = [\n" +
		"  { id: \"first\", oo: [{ k: \"x\", inn: [{ k: \"q\" }, { k: \"y\" }], pick: \"y\" }, { k: \"y\", inn: [{ k: \"q\" }], pick: \"q\" }], pick: \"y\" },\n" +
		"  { id: \"second\", oo: [{ k: \"x\", inn: [{ k: \"q\" }], pick: \"q\" }], pick: \"x\" },\n]\n"),
	"law/p/dev.layer.canon": file("package p\nlayer dev\n\namend aa {\n  second.pick: \"x\"\n  second.oo.x.pick: \"q\"\n  first.pick: \"x\"\n  first.oo.x.pick: \"q\"\n}\n"),
}

// sharedGraph is fsys analyzed with the layer active, and its graph.
func sharedGraph(t *testing.T, fsys mapFS) (fixture, *edit.Graph) {
	t.Helper()
	p, err := build.Open(fsys, "/law", build.Options{Layers: []string{"dev"}})
	if err != nil {
		t.Fatal(err)
	}
	f := analyze(t, p, nil, true)
	g, err := f.Graph(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return f, g
}

// API.md R7, R11, R14 (DECISIONS 483): with the layer active, the base refs it replaced are judged
// in the record now at their place, by Refs and Graph alike, at level 1 and level 2.
func TestGraphParitySharedLayer(t *testing.T) {
	for _, fsys := range []mapFS{sharedParityL1, sharedParityL2} {
		f, g := sharedGraph(t, fsys)
		if got := checkParity(t, f, g); got.edges != len(g.Edges) {
			t.Errorf("%d edges, %d of them to targets", len(g.Edges), got.edges)
		}
	}
}
