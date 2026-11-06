package eval

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

// symSiteCase is a record holding a symbol a default gave provenance def, written at site, and
// the evaluator that reprov'd it.
type symSiteCase struct {
	src       *Evaluator
	root      *value.Record
	def, site *value.Prov
}

// newSymSiteCase reprovs a symbol written at a literal to a default, held by a record.
func newSymSiteCase() symSiteCase {
	c := symSiteCase{src: newEvaluator(check.Bags{}, Options{}), site: &value.Prov{Kind: value.ProvLiteral}}
	c.def = &value.Prov{Kind: value.ProvDefault, Via: &value.Prov{Kind: value.ProvLiteral}}
	sym := c.src.reprov(&value.Symbol{Name: "circle", P: c.site}, c.def)
	c.root = &value.Record{Fields: []value.Value{sym}, Set: []bool{false}}
	return c
}

// EVALUATION.md §13: where a reprov'd symbol was written survives a spread of it, a memo replay and a world copy, warm as cold.
func TestSymbolSiteCarried(t *testing.T) {
	c := newSymSiteCase()
	if got := c.src.writtenAt(c.def); got != c.site {
		t.Fatalf("reprov kept site %p, want %p", got, c.site)
	}
	spread := &value.Prov{Kind: value.ProvSpread, Via: c.def}
	if got := c.src.bareVia(c.src.reprov(c.root.Fields[0], spread).Prov()); got != c.site {
		t.Errorf("a spread's bare case comes via %p, want %p", got, c.site)
	}
	f := c.src.newFreezer(c.root, nil)
	if !f.walk() {
		t.Fatal("the walk met a function value")
	}
	g := f.kept(c.root)
	dst := newEvaluator(check.Bags{}, Options{})
	tab, ok := g.seed(nil)
	if !ok {
		t.Fatal("seed refused")
	}
	dst.thaw(&g, tab)
	if got := dst.bareVia(c.def); got != c.site {
		t.Errorf("after a memo replay the bare case comes via %p, want %p", got, c.site)
	}
	child := newEvaluator(check.Bags{}, Options{})
	child.parent = c.src
	if cf := child.newFreezer(c.root, nil); !cf.walk() || len(cf.marks.sites) != 1 || cf.marks.sites[0].site != c.site {
		t.Errorf("a child's freeze keeps sites %v, want its parent's %p", cf.marks.sites, c.site)
	}
	world := newEvaluator(check.Bags{}, Options{})
	cps, _, ok := copyValues(c.src, world, []value.Value{c.root})
	if !ok {
		t.Fatal("world copy refused")
	}
	if got := world.bareVia(cps[0].(*value.Record).Fields[0].Prov()); got != c.site {
		t.Errorf("after a world copy the bare case comes via %p, want %p", got, c.site)
	}
}

// IMPLEMENTATION-PLAN §7.6: a symbol written elsewhere under the same default fingerprints apart, so no replay keeps its old site.
func TestSymbolSiteFingerprinted(t *testing.T) {
	c, moved := newSymSiteCase(), newSymSiteCase()
	n := source.Pos(len("circle"))
	c.site.Span = source.Span{End: n}
	moved.site.Span = source.Span{Start: 1, End: 1 + n} // the same length elsewhere
	if a, b := c.src.fingerprintOf(c.root), moved.src.fingerprintOf(moved.root); a.fp == b.fp {
		t.Error("a moved case symbol under an unchanged default kept its fingerprint")
	}
}
