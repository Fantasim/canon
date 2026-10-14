package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// Pt's fields in shared.txtar.
const (
	ptX = iota
	ptY
	ptNote
)

// DECISIONS 373: shared Provs stay as they were when a layer or a spread rewrites a value.
func TestProvShared(t *testing.T) {
	a, err := txtar.ParseFile("testdata/provshare/shared.txtar")
	if err != nil {
		t.Fatal(err)
	}
	b := runBuildWith(t, fromArchive(t, a), eval.Options{Layers: []string{"dev"}}, nil)
	rec := func(name string) *value.Record {
		r, ok := b.values[eval.Root{Pkg: "a", Name: name}].(*value.Record)
		if !ok {
			t.Fatalf("root %s: want a record value", name)
		}
		return r
	}
	made, moved, plain := rec("a"), rec("b"), rec("c")
	sited, ok := b.values[eval.Root{Pkg: "a", Name: "d"}].(*value.List)
	if !ok || len(sited.Elems) != 2 {
		t.Fatal("root d: want a list of two records")
	}
	sharedFrame(t, b, made)
	spreadKept(t, moved)
	for _, el := range sited.Elems {
		spreadSited(t, b, plain, el.(*value.Record))
	}
	sharedVia(t, b, plain)
}

// sharedFrame: a.x and a.y were computed in mk's frame and share its ProvExt; the layer that set
// a.x gave it its own provenance, and the one a.y still holds carries no layer.
func sharedFrame(t *testing.T, b *build, made *value.Record) {
	t.Helper()
	x, y := made.Fields[ptX], made.Fields[ptY]
	hist := b.ev.History(made, x)
	if len(hist) != 2 || x.Prov().Kind != value.ProvLayer || x.Prov().Layer() != "dev" {
		t.Fatalf("a.x: want a layer over the computed value, got %d values, %s", len(hist), b.provText(x.Prov()))
	}
	was, yp := hist[1].Prov(), y.Prov()
	if was.Ext == nil || was.Ext != yp.Ext || made.P.Ext != yp.Ext {
		t.Errorf("a's values computed in mk's frame do not share its ProvExt: %s / %s", b.provText(was), b.provText(yp))
	}
	for _, p := range []*value.Prov{was, yp} {
		if p.Kind != value.ProvComputed || p.Layer() != "" || p.Via != nil || len(p.Stack()) != 1 || p.Stack()[0].Fn != "mk" {
			t.Errorf("a shared provenance changed: %s", b.provText(p))
		}
	}
}

// spreadKept: b's spread copy of x has a provenance of its own, via a.x, sharing moved's frame
// ProvExt with b and b.y, which come via nothing.
func spreadKept(t *testing.T, moved *value.Record) {
	t.Helper()
	x, y := moved.Fields[ptX].Prov(), moved.Fields[ptY].Prov()
	if x.Kind != value.ProvSpread || x.Via == nil || x.Ext != moved.P.Ext || len(x.Stack()) != 1 || x.Stack()[0].Fn != "moved" {
		t.Errorf("b.x: want a spread via a.x in moved, got kind %d via %v", x.Kind, x.Via)
	}
	if y.Ext == nil || y.Ext != moved.P.Ext || y.Via != nil || moved.P.Via != nil {
		t.Errorf("b and b.y do not share moved's frame ProvExt without a via")
	}
}

// spreadSited: the spreads a top-level lambda runs start from one shared site Prov; each copy
// comes via the field of c it copies, so none rewrote the shared one.
func spreadSited(t *testing.T, b *build, plain, sited *value.Record) {
	t.Helper()
	for i, f := range sited.Fields {
		if p := f.Prov(); i != ptY && (p.Kind != value.ProvSpread || p.Via != plain.Fields[i].Prov() || p.Ext != nil) {
			t.Errorf("d field %d: want a spread via c's, got %s", i, b.provText(p))
		}
	}
}

// sharedVia: c.y and c.note, left to their defaults, need no ProvExt and come via c; c.x's
// layer leaves the default it replaced as it was.
func sharedVia(t *testing.T, b *build, plain *value.Record) {
	t.Helper()
	x, y, note := plain.Fields[ptX], plain.Fields[ptY].Prov(), plain.Fields[ptNote].Prov()
	hist := b.ev.History(plain, x)
	if len(hist) != 2 {
		t.Fatalf("c.x: want a layer over its default, got %d values", len(hist))
	}
	was := hist[1].Prov()
	for _, p := range []*value.Prov{was, y, note} {
		if p.Kind != value.ProvDefault || p.Via != plain.P || p.Layer() != "" || p.Ext != nil {
			t.Errorf("c's defaults are not plain defaults via c: %s", b.provText(p))
		}
	}
	if x.Prov().Layer() != "dev" || x.Prov() == was {
		t.Errorf("c.x's layer wrote the default it replaced: %s", b.provText(x.Prov()))
	}
}
