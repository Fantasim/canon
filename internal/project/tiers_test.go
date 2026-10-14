package project_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

const tiersProject = `project acme {
  canon: "0.1"
  tiers {
    ids: [items.vocab, header]
    data: [items, services]
  }
}
`

// GRAMMAR.md §7.1 (DECISIONS 380): a path places the packages at and below it, the longest winning.
func TestTiersOf(t *testing.T) {
	var set source.FileSet
	src, err := set.Add(project.FileName, "/p/project.canon", []byte(tiersProject))
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(src, diag.NewBag(&set, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Tiers.Declared() || len(p.Tiers.List) != 2 || p.Tiers.List[1].Name != "data" {
		t.Fatalf("tiers = %+v", p.Tiers)
	}
	cases := []struct {
		pkg, path string
		tier      int
		ok        bool
	}{
		{pkg: "items", path: "items", tier: 1, ok: true},
		{pkg: "items.weapons", path: "items", tier: 1, ok: true},
		{pkg: "items.vocab", path: "items.vocab", ok: true},
		{pkg: "items.vocab.extra", path: "items.vocab", ok: true},
		{pkg: "services.admin", path: "services", tier: 1, ok: true},
		{pkg: "header", path: "header", ok: true},
		{pkg: "itemsx"},
		{pkg: "headers.a"},
		{pkg: ""},
	}
	for _, c := range cases {
		tier, path, ok := p.Tiers.Of(c.pkg)
		if tier != c.tier || path != c.path || ok != c.ok {
			t.Errorf("Of(%q) = %d, %q, %v; want %d, %q, %v", c.pkg, tier, path, ok, c.tier, c.path, c.ok)
		}
	}
}

// GRAMMAR.md §7.1 (DECISIONS 380): no `tiers`, or `tiers {}`, declares no tier and places nothing.
func TestTiersZero(t *testing.T) {
	var none project.Tiers
	if _, _, ok := none.Of("items"); ok || none.Declared() {
		t.Error("the zero Tiers places a package")
	}
}
