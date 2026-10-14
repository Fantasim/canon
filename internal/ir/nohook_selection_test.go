package ir_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	noHookFixture = "testdata/textdecoders_targets/*.txtar"
	noHookOwner   = "a"
	noHookDep     = "d"
	noHookWant    = ": Link Step.hop"
)

// TestNoHookSelection is DECISIONS 363 and 328: NoHook is filled only for the packages a selected package imports, the only ones an importer reads, so whatever the selection (all, {a}, {a, d}) a's decoders are identical in every target, and every go or cpp data emit reachable through a built package's Imports has the same NoHook ([Link Step.hop] for d).
func TestNoHookSelection(t *testing.T) {
	selections := [][]string{nil, {noHookOwner}, {noHookOwner, noHookDep}}
	var wantDecoders, wantHooks string
	for i, sel := range selections {
		decoders, hooks := noHookObserve(t, sel)
		if decoders == "" || !strings.Contains(hooks, noHookWant) {
			t.Fatalf("selection %v observed nothing useful: decoders %q, hooks %q", sel, decoders, hooks)
		}
		if i == 0 {
			wantDecoders, wantHooks = decoders, hooks
			continue
		}
		if decoders != wantDecoders {
			t.Errorf("selection %v: a's decoders differ:\n%s\nwant:\n%s", sel, decoders, wantDecoders)
		}
		if hooks != wantHooks {
			t.Errorf("selection %v: NoHook through Imports differs:\n%s\nwant:\n%s", sel, hooks, wantHooks)
		}
	}
}

// noHookObserve builds the fixture over sel and reports a's decoders per emit and the NoHook of every go and cpp data emit its Imports reach.
func noHookObserve(t *testing.T, sel []string) (decoders, hooks string) {
	t.Helper()
	cases, err := golden.Load(noHookFixture, golden.Expected(planFile))
	if err != nil || len(cases) == 0 {
		t.Fatalf("fixture: %v", err)
	}
	w := newWorld(t)
	for _, f := range cases[0].Archive.Files {
		if f.Name != planFile {
			w.add(t, f.Name, f.Data)
		}
	}
	w.calls = w.fixtureCalls
	var dec, hk []string
	for _, p := range w.build(t, sel...) {
		if p.Name == noHookOwner {
			dec = ownerDecoders(w, p)
		}
		hk = append(hk, importedNoHooks(p)...)
	}
	slices.Sort(hk)
	return strings.Join(dec, "\n"), strings.Join(slices.Compact(hk), "\n")
}

// ownerDecoders are p's decoders per emit.
func ownerDecoders(w *world, p *ir.Package) []string {
	var dec []string
	for _, e := range p.Emits {
		dec = append(dec, e.Out+": "+targetDecoders(ir.CopyOf(w.proj, p, e), e))
	}
	return dec
}

// importedNoHooks are the NoHook lines of the go and cpp data emits p's Imports reach.
func importedNoHooks(p *ir.Package) []string {
	var hk []string
	for _, ref := range p.Imports {
		for _, e := range ref.Emits {
			if e.Mode == ir.ModeData && (e.Target == ir.TargetGo || e.Target == ir.TargetCpp) {
				hk = append(hk, ref.Name+" "+e.Out+": "+strings.Join(e.NoHook, " "))
			}
		}
	}
	return hk
}
