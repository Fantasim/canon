package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"golang.org/x/tools/txtar"
)

const (
	spreadFixture = "testdata/prov/spread_composites.txtar"
	ownedFixture  = "testdata/prov/spread_owned.txtar"
)

// EVALUATION.md §13, §4.2: each composite a spread copies is `spread` via its origin; its parts are the source's.
func TestSpreadCopiedCompositeOrigin(t *testing.T) {
	a, err := txtar.ParseFile(spreadFixture)
	if err != nil {
		t.Fatal(err)
	}
	p := fromArchive(t, a)
	b := runBuildWith(t, p, eval.Options{}, jsonLoader(t, p, a))
	base, cp := rootRecord(t, b, "base"), rootRecord(t, b, "copy")
	for _, name := range []string{"tags", "lim", "inner", "tiers", "nodes", "picks", "pick"} {
		src, got := fieldValue(base, name), fieldValue(cp, name)
		p := got.Prov()
		if p == nil || p.Kind != value.ProvSpread || p.Via != src.Prov() {
			t.Errorf("copy.%s: provenance %+v, want spread via base.%s's %+v", name, p, name, src.Prov())
		}
		if got.CanonText() != src.CanonText() {
			t.Errorf("copy.%s = %s, want base's %s", name, got.CanonText(), src.CanonText())
		}
	}
	list, ok := fieldValue(cp, "tags").(*value.List)
	if !ok || list.Elems[0] != fieldValue(base, "tags").(*value.List).Elems[0] {
		t.Errorf("copy.tags shares no element with base.tags")
	}
}

// EVALUATION.md §4.2, DECISIONS 199, 410: a spread copy's entries and ref stay owned by the record its source holds.
func TestSpreadCopyOwners(t *testing.T) {
	a, err := txtar.ParseFile(ownedFixture)
	if err != nil {
		t.Fatal(err)
	}
	p := fromArchive(t, a)
	b := runBuildWith(t, p, eval.Options{}, jsonLoader(t, p, a))
	for _, c := range []struct{ root, field, owner string }{
		{"base", "many", "base"}, {"copy", "many", "base"}, {"again", "many", "base"}, {"over", "many", "over"},
		{"base", "h", "base"}, {"copy", "h", "base"}, {"again", "h", "base"}, {"over", "h", "base"},
	} {
		rec, ok := fieldValue(rootRecord(t, b, c.root), c.field).(*value.Record)
		if !ok {
			t.Fatalf("%s.%s: no record", c.root, c.field)
		}
		want := fieldValue(rootRecord(t, b, c.owner), c.field)
		for i, got := range ownersOf(rec) {
			if got != want {
				t.Errorf("%s.%s: owner %d is not %s.%s", c.root, c.field, i, c.owner, c.field)
			}
		}
		if c.root != c.owner && rec.P.Kind != value.ProvSpread {
			t.Errorf("%s.%s: provenance %v, want spread", c.root, c.field, rec.P.Kind)
		}
	}
	for _, root := range []string{"g", "g2"} {
		ms, ok := fieldValue(rootRecord(t, b, root), "ms").(*value.Table)
		if !ok || len(ms.Entries) != 1 {
			t.Fatalf("%s.ms: no one-entry table", root)
		}
		for i, got := range ownersOf(ms.Entries[0]) {
			if got != ms.Entries[0] {
				t.Errorf("%s.ms.m1: owner %d is not %s.ms.m1", root, i, root)
			}
		}
	}
	ta := tableEntry(t, b, "ta")
	for _, root := range []string{"ta", "tc"} {
		for i, got := range ownersOf(tableEntry(t, b, root)) {
			if got != ta {
				t.Errorf("%s.a: owner %d is not ta.a, the instance tc.a also is", root, i)
			}
		}
	}
}

// tableEntry is the one entry of the table root name.
func tableEntry(t *testing.T, b *build, name string) *value.Record {
	t.Helper()
	tab, ok := b.values[eval.Root{Pkg: "a", Name: name}].(*value.Table)
	if !ok || len(tab.Entries) != 1 {
		t.Fatalf("a.%s: no one-entry table", name)
	}
	return tab.Entries[0]
}

// ownersOf is the owner of each entry of rec's items and of its pick.
func ownersOf(rec *value.Record) []value.Value {
	var out []value.Value
	if items, ok := fieldValue(rec, "items").(*value.List); ok {
		for _, el := range items.Elems {
			if en, isRec := el.(*value.Record); isRec && en.Ident != nil {
				out = append(out, en.Ident.Owner)
			}
		}
	}
	if ref, ok := fieldValue(rec, "pick").(*value.Ref); ok {
		out = append(out, ref.Owner)
	}
	return out
}

func rootRecord(t *testing.T, b *build, name string) *value.Record {
	t.Helper()
	rec, ok := b.values[eval.Root{Pkg: "a", Name: name}].(*value.Record)
	if !ok {
		t.Fatalf("a.%s: no record", name)
	}
	return rec
}

func fieldValue(rec *value.Record, name string) value.Value {
	for i, f := range verify.Fields(rec.T) {
		if f.Name == name {
			return rec.Fields[i]
		}
	}
	return nil
}
