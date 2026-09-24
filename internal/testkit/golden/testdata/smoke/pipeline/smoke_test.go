// Package smoke is copied beside a temporary copy of examples/pipeline/expected/go by `make
// goldens-vet` (DECISIONS 201): it never lives in expected/, which holds only compiler output
// (M2 acceptance 3, IMPLEMENTATION-PLAN.md §6 M2). data/potions.json is copied there too, from
// the golden itself (Makefile's goldens-vet), so this package keeps no hand copy of it.
// stale/potions.json is its own fixture: a schema no real build ever writes.
package smoke

import (
	"reflect"
	"strings"
	"testing"

	"example.com/potions"
)

// staleSchema is a fingerprint that never matches the real potions.json's (CPP-06's Go side).
const staleSchema = "pipeline.Potion@00000000"

// callGetters calls every exported, no-argument method of v: the generated code's own
// convention for a getter (CODEGEN.md), so a decode gap or a nil slice panics here.
func callGetters(t *testing.T, v any) {
	t.Helper()
	if v == nil {
		return
	}
	rv := reflect.ValueOf(v)
	rt := rv.Type()
	for i := range rt.NumMethod() {
		m := rt.Method(i)
		if m.Type.NumIn() != 1 || m.Type.NumOut() == 0 {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s.%s panicked: %v", rt, m.Name, r)
				}
			}()
			rv.Method(i).Call(nil)
		}()
	}
}

// TestSmoke loads potions.json through the generated data-mode loader (LoadPipelineSnapshot,
// then Store.Reload) and reads every value through its generated getters (M2 acceptance 3).
func TestSmoke(t *testing.T) {
	snap, err := potions.LoadPipelineSnapshot("data")
	if err != nil {
		t.Fatal(err)
	}
	callGetters(t, snap)
	rows := snap.Potions()
	if rows.Len() == 0 {
		t.Fatal("no potions loaded")
	}
	for row := range rows.All() {
		callGetters(t, row)
	}
	if err := potions.Store.Reload("data"); err != nil {
		t.Fatal(err)
	}
	callGetters(t, potions.Store.Current())
	if _, ok := rows.Find("II_POT_NONE"); ok {
		t.Error("Find(II_POT_NONE) found an absent key")
	}
}

// TestStale proves a schema mismatch is refused naming both fingerprints, the current snapshot
// stays after a failed Reload, and a missing file also fails without changing it (CPP-06's Go
// side, M2 acceptance 5).
func TestStale(t *testing.T) {
	if err := potions.Store.Reload("data"); err != nil {
		t.Fatal(err)
	}
	snap := potions.Store.Current()
	if snap == nil {
		t.Fatal("no snapshot after Reload(data)")
	}
	err := potions.Store.Reload("stale")
	if err == nil {
		t.Fatal("Reload(stale) succeeded, want a schema mismatch error")
	}
	if !strings.Contains(err.Error(), staleSchema) || !strings.Contains(err.Error(), potions.PotionsSchema) {
		t.Errorf("Reload(stale) error %q: want both fingerprints", err)
	}
	if potions.Store.Current() != snap {
		t.Error("the snapshot changed after a failed Reload(stale)")
	}
	if err := potions.Store.Reload("missing"); err == nil {
		t.Fatal("Reload(missing) succeeded, want an error")
	}
	if potions.Store.Current() != snap {
		t.Error("the snapshot changed after a failed Reload(missing)")
	}
}
